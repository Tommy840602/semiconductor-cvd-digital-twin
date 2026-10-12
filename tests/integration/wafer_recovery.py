"""Isolated full transport, authorization and recovery workflow acceptance.
CVD_COMPOSE=.runtime/recovery-compose.json python3 tests/integration/wafer_recovery.py
"""
import os,json,time,uuid,pathlib,subprocess,concurrent.futures,requests
os.environ.setdefault('CVD_COMPOSE','.runtime/recovery-compose.json')
from rbac import Client,sql,DC
PWD=pathlib.Path('.runtime/secrets/test-leader-password').read_text().strip()
UI_PASSWORD='Cvd-Recovery-Test-2026!'
def wait(fn,timeout=80):
 end=time.monotonic()+timeout
 while time.monotonic()<end:
  try:value=fn()
  except requests.exceptions.RequestException:value=None
  if value:return value
  time.sleep(.4)
 raise AssertionError('Timed out: '+str(fn))
def command(c,kind,pm='',lot='',parameters=None):
 cid=str(uuid.uuid4());c.call('/commands',dict(commandId=cid,type=kind,targetId=pm,lotId=lot,parameters=parameters or {}))
 r=wait(lambda:(lambda r:r if r['status']!='received' else None)(c.call('/commands/'+cid)))
 assert r['status']=='succeeded',r
 return cid
def main():
 leader=Client();leader.login('cvd.leader',PWD)
 assert leader.call('/auth/me')['department'] is None
 clients={};ids={}
 for name,role,dept in [('ee','Engineer','EE'),('pe','Engineer','PE'),('op','Operator','EE'),('unset','Engineer',None)]:
  username='recovery.'+name
  existing=[u for u in leader.call('/users') if u['username']==username]
  u=existing[0] if existing else leader.call('/users',dict(username=username,password=UI_PASSWORD,role=role,department=dept))
  ids[name]=u['id'];clients[name]=Client();clients[name].login(username,UI_PASSWORD)
 ee,pe=clients['ee'],clients['pe']
 leader.call('/users/'+ids['unset']+'/department',{'department':'EE'});clients['unset'].call('/auth/me',code=401)
 assert clients['unset'].login('recovery.unset',UI_PASSWORD)['role']=='Engineer'
 leader.call('/users/'+ids['unset']+'/department',{'department':None});clients['unset'].login('recovery.unset',UI_PASSWORD)
 for c in [leader,pe,clients['op'],clients['unset']]:c.call('/recoveries',{},code=403)
 for c in [leader,ee,pe,clients['op']]:
  for kind in ['rework','wafer-supplement','cancel-supplement']:c.call('/commands',dict(commandId=str(uuid.uuid4()),type=kind,parameters={'approvalId':'fake'}),code=403)
 command(leader,'resume');command(leader,'configure',parameters={'stepSeconds':{'preheat':2,'stabilize':2,'deposition':20,'purge':2}})
 lot='RECOVERY-'+uuid.uuid4().hex[:6]
 for i in range(1,4):command(leader,'start',lot=lot+str(i),parameters={'lp':'LP'+str(i),'ll':'LL'+str(i)})
 def growing():
  s=leader.call('/equipment');p=s['chambers'][0]
  return p if p.get('waferId') and p.get('processTelemetry') and p['processTelemetry']['thicknessNm']>1 else None
 p=wait(growing);command(leader,'fault',p['id']);s=leader.call('/equipment');p=s['chambers'][0]
 body={'pmId':p['id'],'stepSeconds':[0,0,3,1],'targetThicknessNm':p['processTelemetry']['thicknessNm']+p['processTelemetry']['rootDepositionRateNmPerSecond']*3,'troubleshooting':'EE 已檢查沉積偏差並確認補做秒數'}
 for steps in [[0,0,0,0],[1.5,0,1,0],[3600,1,0,0],[-1,0,0,0]]:ee.call('/recoveries',{**body,'stepSeconds':steps},code=400)
 d=ee.call('/recoveries',body);rid=d['id'];baseline=d['versions'][0]['prescription']['baselineThicknessNm'];wafer=d['wafer_id'];print('draft baseline',round(baseline,6),flush=True)
 ee.call('/recoveries',body,code=409);ee.call('/recoveries/'+rid+'/submit',{'revision':99},code=409)
 d=ee.call('/recoveries/'+rid+'/submit',{'revision':1});ee.call('/recoveries/'+rid+'/approve',{'revision':1},code=403)
 # Even a later department change cannot authorize self-review; the old session is revoked.
 leader.call('/users/'+ids['ee']+'/department',{'department':'PE'});ee.call('/auth/me',code=401);ee.login('recovery.ee',UI_PASSWORD);ee.call('/recoveries/'+rid+'/approve',{'revision':1},code=403)
 leader.call('/users/'+ids['ee']+'/department',{'department':'EE'});ee.login('recovery.ee',UI_PASSWORD)
 pe.call('/recoveries/'+rid+'/return',{'revision':1,'reason':'請加入吹掃並確認精確秒數'})
 d=ee.call('/recoveries/'+rid+'/revise',{**body,'revision':1});assert d['revision']==2 and len(d['versions'])==2
 ee.call('/recoveries/'+rid+'/submit',{'revision':2});d=pe.call('/recoveries/'+rid+'/approve',{'revision':2});assert len(d['intents'])==1
 pe.call('/recoveries/'+rid+'/approve',{'revision':2});assert len(pe.call('/recoveries/'+rid)['intents'])==1
 time.sleep(2);d=ee.call('/recoveries/'+rid);assert d['status']=='approved_waiting' and '故障' in d['reason'];assert leader.call('/equipment')['chambers'][0]['processTelemetry']['thicknessNm']==baseline
 # Unpublished approval can be withdrawn atomically, then requires a new version/review.
 d=ee.call('/recoveries/'+rid+'/withdraw',{'revision':2});assert d['status']=='withdrawn'
 d=ee.call('/recoveries/'+rid+'/revise',{**body,'revision':2});ee.call('/recoveries/'+rid+'/submit',{'revision':3});pe.call('/recoveries/'+rid+'/approve',{'revision':3})
 command(leader,'clear',p['id']);d=wait(lambda:(lambda d:d if d['status'] in ['completed','failed'] else None)(ee.call('/recoveries/'+rid)))
 assert d['status']=='completed',d
 result=d['intents'][-1]['result']['result'];assert result['baselineThicknessNm']==baseline and result['thicknessNm']>baseline and result['elapsedSeconds']==4
 def returned():
  s=leader.call('/equipment');l=next(l for l in s['lots'] if l['id']==lot+'1');w=next(w for w in l['wafers'] if w['id']==wafer);return s if w['location']==l['lp'] else None
 s=wait(returned);assert s['dispatchPaused'] and next(l for l in s['lots'] if l['id']==lot+'1')['paused']
 assert all(len([w for l in s['lots'] for w in l['wafers'] if w['location']==ll['id']])<=1 for ll in s['locks'])
 assert any(e['action']=='executing' for e in d['events']);assert any(e['action']=='completed' for e in d['events'])
 wait(lambda:any(l['id']!=lot+'1' and any(w['result']=='success' for w in l['wafers']) for l in leader.call('/equipment')['lots']))
 pathlib.Path('.runtime/recovery-flow-evidence.json').write_text(json.dumps(d,indent=2))
 print('PASS EE/PE, frozen revisions, self-review rejection, interlock auto-start, cumulative growth and original-slot return',flush=True)
 # Second wafer: no deposition preserves thickness, quality failure stays held.
 command(leader,'resume',lot=lot+'1');p=wait(growing);command(leader,'fault',p['id']);p=leader.call('/equipment')['chambers'][0]
 b={**body,'stepSeconds':[1,0,0,1],'targetThicknessNm':100};d=ee.call('/recoveries',b);id2=d['id'];ee.call('/recoveries/'+id2+'/submit',{'revision':1});pe.call('/recoveries/'+id2+'/approve',{'revision':1});command(leader,'clear',p['id'])
 d=wait(lambda:(lambda d:d if d['status']=='failed' else None)(ee.call('/recoveries/'+id2)));res=d['intents'][0]['result']['result'];assert res['deltaThicknessNm']==0 and res['baselineThicknessNm']==res['thicknessNm']
 # Third attempt preserves original root rate, restart requires the separate recovery clock.
 d=ee.call('/recoveries',{**body,'stepSeconds':[0,0,12,0],'targetThicknessNm':100});id3=d['id'];ee.call('/recoveries/'+id3+'/submit',{'revision':1});pe.call('/recoveries/'+id3+'/approve',{'revision':1});wait(lambda:ee.call('/recoveries/'+id3)['status']=='executing')
 subprocess.run(DC+['stop','simulator'],check=True,stdout=subprocess.DEVNULL)
 subprocess.run(DC+['start','simulator'],check=True,stdout=subprocess.DEVNULL)
 wait(lambda:leader.call('/equipment').get('paused'))
 before=leader.call('/equipment')['chambers'][0]['processTelemetry'];assert before['rootDepositionRateNmPerSecond']==p['processTelemetry']['rootDepositionRateNmPerSecond']
 time.sleep(2);after=leader.call('/equipment')['chambers'][0]['processTelemetry'];assert before['thicknessNm']==after['thicknessNm']
 command(leader,'recovery-clock');command(leader,'fault','PM1');d=wait(lambda:(lambda d:d if d['status']=='failed' else None)(ee.call('/recoveries/'+id3)));assert d['intents'][0]['result']['result']['thicknessNm']>=before['thicknessNm']
 ee.call('/recoveries/'+id3+'/withdraw',{'revision':1},code=409)
 print('PASS no-deposition zero growth, failed quality hold, checkpoint resume, repeated fault and unchanged root rate',flush=True)
 # Leave one EE submitted proposal for browser review; do not mutate production accounts.
 d=ee.call('/recoveries',{**body,'stepSeconds':[0,0,1,1],'targetThicknessNm':10});ee.call('/recoveries/'+d['id']+'/submit',{'revision':1})
 pathlib.Path('.runtime/recovery-ui-request').write_text(d['id'])
if __name__=='__main__':main()
