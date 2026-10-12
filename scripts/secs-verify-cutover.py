#!/usr/bin/env python3
"""Read-only cutover verification against a restored or production deployment."""
import argparse,datetime,hashlib,json,pathlib,subprocess,time,requests
p=argparse.ArgumentParser();p.add_argument('compose');p.add_argument('backup');p.add_argument('--port',type=int,choices=[18000,18080,22000],required=True);p.add_argument('--secs',action='store_true');p.add_argument('--output',required=True);a=p.parse_args()
c=json.loads(pathlib.Path(a.compose).read_text());assert c['name'] in ('cvd-twin','cvd-secs-restore');dc=['docker','compose','-f',a.compose]
base=f'http://127.0.0.1:{a.port}/api';session=requests.Session()
def get(path):
 r=session.get(base+path,timeout=10);r.raise_for_status();return r.json()
for _ in range(120):
 try:
  csrf=get('/auth/csrf')['token'];password=pathlib.Path(c['secrets']['leader_password']['file']).read_text().strip()
  r=session.post(base+'/auth/login',json={'username':'cvd.leader','password':password},headers={'X-CSRF-Token':csrf},timeout=10);r.raise_for_status();break
 except requests.RequestException:time.sleep(2)
else:raise RuntimeError('login unavailable')
for _ in range(120):
 fleet=get('/equipment');now=time.time()
 if len(fleet)==10 and all(m.get('sourceTime') and now-datetime.datetime.fromisoformat(m['sourceTime'].replace('Z','+00:00')).timestamp()<8 and (not a.secs or m.get('communicationQuality')=='good') for m in fleet):break
 time.sleep(2)
else:raise RuntimeError('ten fresh equipment snapshots unavailable')
backup=pathlib.Path(a.backup);count=0
fields=['equipmentId','recipeVersion','modelSeed','modelVersion','dispatchPaused','tick','lots','locks','chambers','atm','vac','recipeSteps','processSeconds','commands','recoveries','lastDispatchOrder','nextLoadLock']
for n in range(1,11):
 id=f'TEL-CVDTIN-08A-{n:02}';service='simulator' if n==1 else f'simulator-{n:02}'
 before=json.loads((backup/'states-original'/(id+'.json')).read_text())
 state=json.loads(subprocess.check_output(dc+['exec','-T',service,'cat','/data/state.json'],text=True))
 assert state['paused'],id+' not paused'
 for field in fields:assert before.get(field)==state.get(field),id+' changed '+field
 assert state['epoch']==before['epoch'] and state['sequence']>=before['sequence'],id+' identity reset'
 api=get('/equipment/'+id);assert api['equipmentId']==id and api['lots']==before['lots']
 if a.secs:assert api.get('communicationQuality')=='good',id+' stale API cache'
 count+=sum(len(l['wafers']) for l in state['lots'])
def sql(q):return subprocess.check_output(dc+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-Atc',q],text=True)
users=sql('select row_to_json(u) from app_users u order by username');digest=hashlib.sha256(users.encode()).hexdigest()
record=backup/'account-digest.txt'
if record.exists():assert record.read_text()==digest,'account data changed'
else:record.write_text(digest)
result={'equipment':10,'wafersPreserved':count,'allPaused':True,'businessStateUnchanged':True,'accountsUnchanged':True,'accountCount':len(users.splitlines()),'protocol':'SECS/GEM + Sparkplug B' if a.secs else 'restored OPC UA','freshSnapshots':True}
pathlib.Path(a.output).write_text(json.dumps(result,indent=2));print(json.dumps(result))
