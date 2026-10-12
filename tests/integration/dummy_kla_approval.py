import requests,time,uuid,json,subprocess
from pathlib import Path
assert json.loads(Path('.runtime/dummy-test-compose.json').read_text())['name']=='cvd-dummy-approval-test'
base='http://127.0.0.1:18023/api';eq='TEL-CVDTIN-08A-01';compose=['docker','compose','-f','.runtime/dummy-test-compose.json']
def wait(fn,timeout=180):
 end=time.time()+timeout
 while time.time()<end:
  try:
   v=fn()
   if v:return v
  except requests.RequestException:pass
  time.sleep(1)
 raise RuntimeError('condition timeout')
wait(lambda:requests.get(base+'/auth/csrf').status_code==200)
subprocess.run(compose+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-c',"UPDATE app_users SET department='PE' WHERE username='cvd.leader'"],check=True,stdout=subprocess.DEVNULL)
s=requests.Session();csrf=s.get(base+'/auth/csrf').json()['token'];r=s.post(base+'/auth/login',headers={'X-CSRF-Token':csrf},json={'username':'cvd.leader','password':Path('.runtime/secrets/leader-password').read_text().strip()});assert r.status_code==200,r.status_code;s.headers['X-CSRF-Token']=r.json()['csrf']
def api(path,body=None,code=200):
 r=s.get(base+path) if body is None else s.post(base+path,json=body)
 assert r.status_code==code,(path,r.status_code,r.text[:250])
 return r.json() if r.text else None
def state():return api('/equipment/'+eq)
def command(kind,p=None,code=200):
 r=api('/equipment/'+eq+'/commands',{'commandId':str(uuid.uuid4()),'type':kind,'parameters':p or {}},code)
 if code!=200:return r
 result=wait(lambda:(x if x['status'] in ['succeeded','rejected'] else False) if (x:=api('/equipment/'+eq+'/commands/'+r['id'])) else False)
 assert result['status']=='succeeded',(kind,result)
 return result
def maint(kind,extra=None,code=200):
 m=state().get('maintenance');p={'maintenanceId':m['id'],'expectedGeneration':m['generation']} if m else {};p.update(extra or {});return command(kind,p,code)
wait(lambda:state().get('dummyApprovalModelVersion')==1)
command('resume')
maint('maintenance-request');wait(lambda:state()['maintenance']['phase']=='maintenance')
maint('maintenance-end');maint('maintenance-carrier-load',{'lp':'LP1'});maint('qualification-start')
wait(lambda:state()['maintenance']['phase']=='passed',300)
qpath='/equipment/'+eq+'/maintenance/qualification'
q=wait(lambda:(q if q['canConfirm'] else False) if (q:=api(qpath)) else False,300)
assert len(q['evidence']['wafers'])==4
maint('maintenance-carrier-restore');maint('resume',code=409)
q=api(qpath);Path('.runtime/dummy-ui-fixture.json').write_text(json.dumps({'state':state(),'qualification':q}));q=api(qpath,{'requestId':str(uuid.uuid4()),'decision':'confirm','reason':'four wafer KLA verification','digest':q['digest']});assert q['confirmed'];assert state()['dispatchPaused']
maint('resume');assert state()['maintenance']['phase']=='released'
Path('.runtime/dummy-flow-proof.json').write_text(json.dumps(q,indent=2));print('PASS real four dummy CVD -> KLA -> restore -> PE confirm -> manual resume, no automatic release',flush=True)
