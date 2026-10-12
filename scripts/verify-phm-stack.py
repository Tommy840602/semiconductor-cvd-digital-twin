#!/usr/bin/env python3
"""Synthetic lifecycle only against the isolated cvd-phm-validation project."""
import datetime,json,pathlib,time,uuid,urllib.request,urllib.error,http.cookiejar,subprocess
root=pathlib.Path(__file__).resolve().parents[1];base='http://127.0.0.1:19080/api';eq='TEL-CVDTIN-08A-01';prefix='/equipment/'+eq;dc=['docker','compose','-f',str(root/'.runtime/phm-validation.json')];csrf=''
op=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def api(path,body=None,client=None,token=None):
 req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json','X-CSRF-Token':csrf if token is None else token})
 return json.load((client or op).open(req,timeout=30))
def wait(fn,label,seconds=120):
 start=time.monotonic();last=None
 while time.monotonic()-start<seconds:
  try:
   last=fn()
   if last:return last
  except (OSError,ValueError):pass
  time.sleep(1)
 raise AssertionError(label+' timed out')
def state():return api(prefix)
def command(kind,parameters=None,target='',identifier=None):
 body={'commandId':identifier or str(uuid.uuid4()),'type':kind,'parameters':parameters or {},'targetId':target,'expiresAt':(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(minutes=5)).isoformat()}
 submitted=datetime.datetime.now(datetime.timezone.utc)
 r=api(prefix+'/commands',body)
 def done():
  r=api(prefix+'/commands/'+body['commandId']);return r if r['status'] in ['succeeded','rejected'] else None
 r=wait(done,kind)
 assert r['status']=='succeeded',(kind,r)
 wait(lambda:datetime.datetime.fromisoformat(state()['sourceTime'].replace('Z','+00:00'))>submitted,kind+' snapshot')
 return body

def maint(kind,extra=None):
 m=state().get('maintenance');p={} if not m else {'maintenanceId':m['id'],'expectedGeneration':m['generation']};p.update(extra or {});return command(kind,p)
wait(lambda:api('/auth/csrf'),'backend startup',600)
csrf=api('/auth/csrf')['token'];csrf=api('/auth/login',{'username':'cvd.leader','password':(root/'.runtime/phm-test-secrets/leader_password').read_text().strip()})['csrf']
wait(lambda:state().get('communicationQuality')=='good' and state().get('recipeVersion') and (datetime.datetime.now(datetime.timezone.utc)-datetime.datetime.fromisoformat(state()['sourceTime'].replace('Z','+00:00'))).total_seconds()<4,'fresh simulator',300)
# Real endpoint authorization and CSRF checks.
password=uuid.uuid4().hex;username='phm.op.'+uuid.uuid4().hex[:8];api('/users',{'username':username,'password':password,'role':'Operator','department':'EE'})
op2=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()));tok=api('/auth/csrf',client=op2)['token'];tok=api('/auth/login',{'username':username,'password':password},op2,tok)['csrf']
for kind in ['vacuum-recover','vacuum-test','vacuum-service-channel','maintenance-request']:
 try:api(prefix+'/commands',{'commandId':str(uuid.uuid4()),'type':kind},op2,tok);raise AssertionError('Operator accepted')
 except urllib.error.HTTPError as e:assert e.code==403
try:api(prefix+'/commands',{'commandId':str(uuid.uuid4()),'type':'vacuum-test'},token='');raise AssertionError('CSRF accepted')
except urllib.error.HTTPError as e:assert e.code==403
assert len(api(prefix+'/vacuum?pumpId=LL1',client=op2)['locks'])==3
command('resume')
body={'commandId':str(uuid.uuid4()),'type':'start','lotId':eq+'-phm-fixture','parameters':{'lp':'LP1','ll':'LL1'}};api(prefix+'/commands',body)
wait(lambda:state()['locks'][0]['pump']['fault'],'pump failure',90);s=state();assert s['vacuumHold'];p=s['locks'][0]['pump'];params={'cycleId':p['cycle']['id'],'faultGeneration':p['faultGeneration']}
command('vacuum-recover',params,'LL1');wait(lambda:not state()['locks'][0]['pump']['recovering'],'controlled return')
repair=command('vacuum-service-channel',params,'LL1');wait(lambda:not state()['locks'][0]['pump']['channelTesting'],'channel qualification')
def drained():
 s=state();return not s['atm']['waferId'] and not s['vac']['waferId'] and all(not p['waferId'] for p in s['chambers']) and all(not l['waferId'] and l['remaining']==0 for l in s['locks'])
wait(drained,'drain retained PM',120);assert state()['dispatchPaused'];assert state()['locks'][0]['pump']['needsService']
gen=state()['locks'][0]['pump']['serviceGeneration'];api(prefix+'/commands',repair);time.sleep(1);assert state()['locks'][0]['pump']['serviceGeneration']==gen
print('PASS Operator/CSRF guards, real failure, controlled return, channel repair, PM drain and duplicate repair',flush=True)
# Broker reconnection must preserve hold and evidence.
reconnect=datetime.datetime.now(datetime.timezone.utc)
subprocess.run(dc+['restart','mqtt'],check=True,stdout=subprocess.DEVNULL)
wait(lambda:state().get('communicationQuality')=='good' and datetime.datetime.fromisoformat(state()['sourceTime'].replace('Z','+00:00'))>reconnect and (datetime.datetime.now(datetime.timezone.utc)-datetime.datetime.fromisoformat(state()['sourceTime'].replace('Z','+00:00'))).total_seconds()<4,'broker reconnect',180);assert state()['vacuumHold']
maint('maintenance-request',{'selectedPumpIds':['LL1']});wait(lambda:state()['maintenance']['phase']=='maintenance','maintenance');maint('maintenance-end');maint('vacuum-test')
subprocess.run(dc+['restart','simulator'],check=True,stdout=subprocess.DEVNULL)
wait(lambda:state()['maintenance']['pumpTest']['status']=='interrupted','interrupted test',60);assert state()['vacuumHold'];command('recovery-clock')
maint('maintenance-request',{'selectedPumpIds':['LL1']});maint('maintenance-end');maint('vacuum-test');wait(lambda:state()['maintenance']['pumpTest']['status']=='passed','pump retest')
maint('maintenance-carrier-load',{'lp':'LP1'});maint('qualification-start');wait(lambda:state()['maintenance']['phase']=='passed','four PM dummy',240)
maint('maintenance-carrier-restore');maint('resume');assert not state()['vacuumHold'];assert state()['maintenance']['phase']=='released'
# Kafka restart must not reset counters, qualification or replay a completed service.
prior=state();life=prior['locks'][0]['pump']['lifetimeCycles'];generation=prior['locks'][0]['pump']['serviceGeneration']
reconnect=datetime.datetime.now(datetime.timezone.utc)
subprocess.run(dc+['restart','kafka'],check=True,stdout=subprocess.DEVNULL)
wait(lambda:datetime.datetime.fromisoformat(state()['sourceTime'].replace('Z','+00:00'))>reconnect and (datetime.datetime.now(datetime.timezone.utc)-datetime.datetime.fromisoformat(state()['sourceTime'].replace('Z','+00:00'))).total_seconds()<4,'Kafka reconnect',180)
assert state()['locks'][0]['pump']['lifetimeCycles']>=life and state()['locks'][0]['pump']['serviceGeneration']==generation
r=api(prefix+'/vacuum?pumpId=LL1');assert r['counts']['failed']>=1 and r['counts']['interrupted']>=1 and len(r['trend'])>0
assert len(api('/equipment'))==10
proof={'counts':r['counts'],'events':len(r['events']),'trend':len(r['trend']),'pump':state()['locks'][0]['pump'],'maintenance':state()['maintenance']}
(root/'.runtime/phm-stack-proof.json').write_text(json.dumps(proof,indent=2))
print('PASS broker reconnect, restart interruption, full maintenance, pump retest, four PM dummy, manual resume, retained history',flush=True)
