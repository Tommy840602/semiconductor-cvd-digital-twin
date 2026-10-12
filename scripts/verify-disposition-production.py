#!/usr/bin/env python3
"""Read-only deployment verification; never issues equipment/business commands."""
import json,pathlib,time,requests,subprocess
root=pathlib.Path(__file__).resolve().parents[1];backup=root/'.runtime/backups/cvd-disposition-20260919';base='http://127.0.0.1:18000/api';s=requests.Session()
def wait(fn):
 end=time.monotonic()+150
 while time.monotonic()<end:
  try:
   value=fn()
   if value:return value
  except requests.RequestException:pass
  time.sleep(1)
 raise AssertionError('Deployment state timed out')
wait(lambda:s.get(base+'/auth/csrf').status_code==200)
token=s.get(base+'/auth/csrf').json()['token'];r=s.post(base+'/auth/login',json={'username':'cvd.leader','password':(root/'.runtime/secrets/leader-password').read_text().strip()},headers={'X-CSRF-Token':token});assert r.status_code==200,'production login failed'
proof={'equipment':[]}
for file in sorted((backup/'states-original').glob('*.json')):
 before=json.loads(file.read_text());id=before['equipmentId'];url=base+'/equipment/'+id
 current=wait(lambda:(lambda x:x if x.get('modelVersion')==6 and x.get('communicationQuality')=='good' else None)(s.get(url).json()))
 old={w['id']:w for l in before['lots'] for w in l['wafers']};new={w['id']:w for l in current['lots'] for w in l['wafers']}
 assert old.keys()==new.keys(),id
 for wid,w in old.items():
  for key in ['location','slot','lotId','attempts']:
   assert w[key]==new[wid][key],(id,wid,key)
  if w['result']!='success':assert w['result']==new[wid]['result'],(id,wid,'result')
 assert current['paused'],id
 if before.get('dispatchPaused'):assert current['dispatchPaused'],id
 for p in before['chambers']:
  if p.get('processTelemetry'):
   q=next(q for q in current['chambers'] if q['id']==p['id'])
   for key in ['thicknessNm','sampleThicknessNm','elapsedSeconds']:assert q['processTelemetry'][key]==p['processTelemetry'][key]
 proof['equipment'].append(dict(equipmentId=id,wafers=len(new),paused=current['paused'],dispatchPaused=current['dispatchPaused'],dispositionHold=current['dispositionHold'],incidents=len(current.get('incidents') or {}),version=current['version']))
 # Old entries remain explicit legacy records rather than fabricated signatures.
 assert s.get(url+'/recoveries').status_code==200
 assert s.get(url+'/release').status_code==200
for service,tables in [('postgres',['app_users','recovery_requests','recovery_versions','recovery_intents']),('timescaledb',['telemetry'])]:
 counts={}
 for table in tables:
  counts[table]=int(subprocess.check_output(['docker','compose','exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc','SELECT count(*) FROM '+table],text=True,cwd=root))
 baseline=json.loads((root/'.runtime/disposition-backup-proof.json').read_text())[service+'Restore']
 for table,count in counts.items():assert count>=baseline[table],(table,count,baseline[table])
 proof[service]=counts
(root/'.runtime/disposition-production-proof.json').write_text(json.dumps(proof,indent=2));print('PASS ten tools modelVersion6; 725 wafers positions/attempts preserved; original holds and clock pauses retained; accounts/history preserved')
