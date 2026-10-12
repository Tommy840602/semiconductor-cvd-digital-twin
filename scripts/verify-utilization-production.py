#!/usr/bin/env python3
"""Read-only checks after utilization deployment; never submits equipment commands."""
import json,pathlib,requests,time,subprocess
root=pathlib.Path(__file__).resolve().parents[1]
backup=root/'.runtime/backups/cvd-utilization-20260919-complete'
s=requests.Session();base='http://127.0.0.1:18000/api'
def wait(fn,timeout=120):
 end=time.monotonic()+timeout
 while time.monotonic()<end:
  try:
   value=fn()
   if value:return value
  except (requests.RequestException,KeyError,ValueError):pass
  time.sleep(1)
 raise AssertionError('Verification timed out')
wait(lambda:s.get(base+'/auth/csrf').status_code==200)
token=s.get(base+'/auth/csrf').json()['token'];r=s.post(base+'/auth/login',json={'username':'cvd.leader','password':(root/'.runtime/secrets/leader-password').read_text().strip()},headers={'X-CSRF-Token':token});assert r.status_code==200
proof={'tools':[]}
files=sorted((backup/'states-original').glob('*.json'));assert len(files)==10
for file in files:
 before=json.loads(file.read_text());id=before['equipmentId']
 current=wait(lambda:(lambda j:j if j.get('communicationQuality')=='good' else None)(s.get(base+'/equipment/'+id).json()))
 for key in ['lots','recipeVersion','recipeSteps','dispatchPaused','dispositionHold','incidents','recoveries']:
  assert current.get(key)==before.get(key),(id,key)
 assert current['paused'],id
 proof['tools'].append({'equipmentId':id,'wafers':sum(len(l['wafers']) for l in current['lots']),'paused':True,'businessStatePreserved':True})
r=wait(lambda:(lambda j:j if all(m['knownSeconds']>0 for m in j.get('equipment',[])) and len(j.get('equipment',[]))==10 else None)(s.get(base+'/equipment/utilization?period=today').json()))
assert r['fleet']['quality']=='partial' and r['fleet']['utilization'] is None
proof['report']=r
proof['health']=subprocess.check_output(['docker','compose','ps','--format','{{.Service}} {{.State}} {{.Health}}'],text=True,cwd=root)
(root/'.runtime/utilization-production-proof.json').write_text(json.dumps(proof,indent=2))
print(json.dumps({'tools':10,'wafersPreserved':sum(t['wafers'] for t in proof['tools']),'pausedPreserved':True,'allToolsSampling':True}))
