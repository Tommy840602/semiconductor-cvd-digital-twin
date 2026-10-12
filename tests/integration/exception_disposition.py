"""Full SECS → Sparkplug → Kafka exception workflow; isolated project ONLY."""
import json,os,pathlib,subprocess,time,uuid,requests
ROOT=pathlib.Path(__file__).resolve().parents[2]
COMPOSE=ROOT/'.runtime/disposition-compose.json'
assert json.loads(COMPOSE.read_text())['name']=='cvd-disposition-test'
DC=['docker','compose','-f',str(COMPOSE)]
BASE='http://127.0.0.1:22000/api'
IDS=[f'TEL-CVDTIN-08A-{i:02}' for i in range(1,11)]
PASSWORD='Disposition-Test-2026!'
def wait(fn,timeout=90):
 end=time.monotonic()+timeout
 last=None
 while time.monotonic()<end:
  try:
   value=fn()
   if value:return value
  except (requests.RequestException,AssertionError) as ex:last=ex
  time.sleep(.5)
 raise AssertionError(f'Timed out: {last}')
class Client:
 def __init__(self,name,password):
  self.s=requests.Session();self.token=self.s.get(BASE+'/auth/csrf').json()['token'];r=self.call('/auth/login',dict(username=name,password=password));self.token=r['csrf'];self.user=r['user']
 def call(self,path,body=None,code=200):
  r=self.s.request('GET' if body is None else 'POST',BASE+path,json=body,headers={'X-CSRF-Token':self.token},timeout=15)
  assert r.status_code==code,(path,r.status_code,r.text[:400]);return r.json()
 def tool(self,id,path='',body=None,code=200):return self.call('/equipment/'+id+path,body,code)
def sql(q):return subprocess.check_output(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
def command(c,id,kind,pm='',lot='',params=None,code=200):
 cid=str(uuid.uuid4());r=c.tool(id,'/commands',dict(commandId=cid,type=kind,targetId=pm,lotId=lot,parameters=params or {}),code)
 if code!=200:return r
 r=wait(lambda:(lambda x:x if x['status'] in ('succeeded','rejected') else None)(c.tool(id,'/commands/'+cid)));assert r['status']=='succeeded',r;return r

def run():
 wait(lambda:requests.get(BASE+'/auth/csrf').status_code==200,120)
 leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip())
 for name,role,dept in [('disposition.ee','Engineer','EE'),('disposition.pe','Engineer','PE'),('disposition.op','Operator','EE')]:
  if not any(u['username']==name for u in leader.call('/users')):leader.call('/users',dict(username=name,password=PASSWORD,role=role,department=dept))
 users={u['username']:u for u in leader.call('/users')}
 leader.call('/users/'+leader.user['id']+'/department',{'department':'EE'})
 leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip())
 ee=Client('disposition.ee',PASSWORD);pe=Client('disposition.pe',PASSWORD);op=Client('disposition.op',PASSWORD)
 for id in IDS:wait(lambda id=id:leader.tool(id).get('modelVersion')==6)
 id=IDS[0];other=IDS[1];evidence={}
 command(leader,id,'resume')
 recipe=leader.tool(id,'/recipes');r=leader.tool(id,'/recipes',dict(expectedVersion=recipe['currentVersion'],settings={'stepSeconds':dict(preheat=1,stabilize=1,deposition=24,purge=1)}));wait(lambda:leader.tool(id,'/commands/'+r['commandId'])['status']=='succeeded')
 lots=[]
 for n in range(1,4):
  lot=id+'-DISP-'+uuid.uuid4().hex[:6];lots.append(lot);command(leader,id,'start',lot=lot,params={'lp':f'LP{n}','ll':f'LL{n}'})
 def growing():return next((p for p in leader.tool(id)['chambers'] if p.get('waferId') and p.get('processTelemetry',{}).get('thicknessNm',0)>1),None)
 p=wait(growing);command(leader,id,'fault',p['id']);p=next(x for x in leader.tool(id)['chambers'] if x['id']==p['id']);wafer=p['waferId']
 auto=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['wafer_id']==wafer),None));assert auto['owner_id'] is None and auto['revision']==0
 source=p['processTelemetry'];body=dict(incidentId=auto['incident_id'],pmId=p['id'],dispositionType='supplement',cause='模擬沉積中斷',correctiveAction='已檢查並修復氣體供應',verificationMethod='比對壓力與流量',verificationResult='模擬數據恢復正常',stepSeconds=[0,0,3,1],targetThicknessNm=source['thicknessNm']+source['rootDepositionRateNmPerSecond']*3)
 ee.tool(id,'/recoveries',{**body,'verificationResult':''},400)
 op.tool(id,'/recoveries',body,403);pe.tool(id,'/recoveries',body,403)
 d=ee.tool(id,'/recoveries',body);rid=d['id'];rev=d['revision'];assert rid==auto['id']
 ee.tool(other,'/recoveries/'+rid,code=404)
 ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':rev+1},409)
 ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':rev})
 # Identity separation persists across department changes.
 leader.call('/users/'+ee.user['id']+'/department',{'department':'PE'});ee=Client('disposition.ee',PASSWORD)
 ee.tool(id,'/recoveries/'+rid+'/approve',{'revision':rev},403)
 leader.call('/users/'+ee.user['id']+'/department',{'department':'EE'});ee=Client('disposition.ee',PASSWORD)
 pe.tool(id,'/recoveries/'+rid+'/return',{'revision':rev,'reason':'補充驗證後重送'})
 d=ee.tool(id,'/recoveries/'+rid+'/revise',{**body,'revision':rev});rev=d['revision'];ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':rev})
 d=pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':rev});eid=d['intents'][-1]['execution_id'];pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':rev});assert len(pe.tool(id,'/recoveries/'+rid)['intents'])==1
 for kind in ['scrap','rework','wafer-supplement','equipment-release']:command(leader,id,kind,p['id'],code=403)
 command(leader,id,'resume',code=409)
 command(leader,id,'clear',p['id']);command(leader,id,'recovery-clock')
 d=wait(lambda:(lambda d:d if d['status'] in ['awaiting_result','failed'] else None)(ee.tool(id,'/recoveries/'+rid)));assert d['status']=='awaiting_result',d
 t=d['intents'][-1]['result']['result'];assert t['elapsedSeconds']==4 and t['stepSeconds']==[0,0,3,1] and t['baselineThicknessNm']==source['thicknessNm']
 time.sleep(4);s=leader.tool(id);inc=s['incidents'][auto['incident_id']];assert next(w for l in s['lots'] for w in l['wafers'] if w['id']==wafer)['location']==p['id'];assert s['dispositionHold']
 pe.tool(id,'/dispositions/'+rid+'/confirm',dict(revision=rev,executionId=eid,resultVersion=inc['resultVersion']-1),409)
 resultbody=dict(revision=rev,executionId=eid,resultVersion=inc['resultVersion'],reason='PE 已核對膜厚與步驟')
 pe.tool(id,'/dispositions/'+rid+'/confirm',resultbody);pe.tool(id,'/dispositions/'+rid+'/confirm',resultbody)
 wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='resolved')
 s=leader.tool(id);assert s['incidents'][auto['incident_id']]['returned'] and s['dispositionHold']
 view=wait(lambda:(lambda v:v if v['eligible'] else None)(leader.tool(id,'/release')))
 pe.tool(id,'/release',{'generation':view['generation']},409)
 # A participant promoted to Leader is still excluded; test-only SQL changes role.
 sql("UPDATE app_users SET role='Leader',auth_version=auth_version+1 WHERE username='disposition.pe'")
 pe=Client('disposition.pe',PASSWORD);assert not pe.tool(id,'/release')['eligible'];pe.tool(id,'/release',{'generation':view['generation']},409)
 sql("UPDATE app_users SET role='Engineer',auth_version=auth_version+1 WHERE username='disposition.pe'");pe=Client('disposition.pe',PASSWORD)
 leader.tool(id,'/release',{'generation':view['generation']});wait(lambda:not leader.tool(id)['dispositionHold']);wait(lambda:leader.tool(id,'/release')['releases'][0]['status']=='released')
 evidence['supplement']=ee.tool(id,'/recoveries/'+rid);evidence['firstRelease']=leader.tool(id,'/release')['releases'][0]
 print('PASS supplement → PM hold → PE confirmation → original slot → independent Leader release',flush=True)
 # New cycle: PE rejects a passing supplement, same request revised to scrap.
 p=wait(growing);command(leader,id,'fault',p['id']);p=next(x for x in leader.tool(id)['chambers'] if x['id']==p['id']);wafer2=p['waferId'];auto=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['wafer_id']==wafer2),None));source=p['processTelemetry']
 body.update(incidentId=auto['incident_id'],pmId=p['id'],targetThicknessNm=source['thicknessNm']+source['rootDepositionRateNmPerSecond']*3)
 d=ee.tool(id,'/recoveries',body);rid2=d['id'];rev=d['revision'];ee.tool(id,'/recoveries/'+rid2+'/submit',{'revision':rev});pe.tool(id,'/recoveries/'+rid2+'/approve',{'revision':rev});command(leader,id,'clear',p['id'])
 wait(lambda:ee.tool(id,'/recoveries/'+rid2)['status']=='awaiting_result');s=leader.tool(id);inc=s['incidents'][auto['incident_id']];pe.tool(id,'/dispositions/'+rid2+'/reject',dict(revision=rev,executionId=inc['executionId'],resultVersion=inc['resultVersion'],reason='PE 判定需報廢處理'))
 wait(lambda:ee.tool(id,'/recoveries/'+rid2)['status']=='failed')
 d=ee.tool(id,'/recoveries/'+rid2+'/revise',{**body,'revision':rev,'dispositionType':'scrap','stepSeconds':[]});rev=d['revision'];ee.tool(id,'/recoveries/'+rid2+'/submit',{'revision':rev});pe.tool(id,'/recoveries/'+rid2+'/approve',{'revision':rev});wait(lambda:ee.tool(id,'/recoveries/'+rid2)['status']=='awaiting_result')
 s=leader.tool(id);inc=s['incidents'][auto['incident_id']];assert inc['returned'];assert next(w for l in s['lots'] for w in l['wafers'] if w['id']==wafer2)['result']=='scrap'
 pe.tool(id,'/dispositions/'+rid2+'/confirm',dict(revision=rev,executionId=inc['executionId'],resultVersion=inc['resultVersion']));wait(lambda:ee.tool(id,'/recoveries/'+rid2)['status']=='resolved')
 # Another PM incident blocks the entire tool even after wafer disposition resolved.
 empty=next(p for p in leader.tool(id)['chambers'] if not p['waferId']);command(leader,id,'fault',empty['id']);assert not leader.tool(id,'/release')['eligible']
 autoempty=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['prescription']['pmId']==empty['id'] and not r['wafer_id'] and r['owner_id'] is None),None))
 d=ee.tool(id,'/recoveries',{**body,'incidentId':autoempty['incident_id'],'pmId':empty['id'],'dispositionType':'equipment'});erid=d['id'];erev=d['revision'];ee.tool(id,'/recoveries/'+erid+'/submit',{'revision':erev});pe.tool(id,'/recoveries/'+erid+'/approve',{'revision':erev});command(leader,id,'clear',empty['id']);wait(lambda:ee.tool(id,'/recoveries/'+erid)['status']=='awaiting_result');inc=leader.tool(id)['incidents'][autoempty['incident_id']]
 pe.tool(id,'/dispositions/'+erid+'/confirm',dict(revision=erev,executionId=inc['executionId'],resultVersion=inc['resultVersion']));wait(lambda:ee.tool(id,'/recoveries/'+erid)['status']=='resolved')
 # PE department Leader (same uninvolved initial Leader) is also eligible.
 leader.call('/users/'+leader.user['id']+'/department',{'department':'PE'});leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip());view=wait(lambda:(lambda v:v if v['eligible'] else None)(leader.tool(id,'/release')));leader.tool(id,'/release',{'generation':view['generation']});wait(lambda:not leader.tool(id)['dispositionHold'])
 evidence['scrap']=ee.tool(id,'/recoveries/'+rid2);evidence['emptyPM']=ee.tool(id,'/recoveries/'+erid)
 assert all(not leader.tool(other).get('dispositionHold') for other in IDS[1:])
 assert sql("SELECT count(*) FROM disposition_confirmations")=='4'
 assert sql("SELECT count(*) FROM disposition_incidents")== '3'
 evidence['releaseHistory']=leader.tool(id,'/release')['releases'];evidence['isolatedEquipmentIds']=IDS
 (ROOT/'.runtime/disposition-evidence.json').write_text(json.dumps(evidence,indent=2,default=str))
 print('PASS PE rejection → same request scrap → return confirmation; empty PM block; EE and PE Leader release; 10-tool isolation',flush=True)
if __name__=='__main__':run()
