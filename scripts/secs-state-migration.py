#!/usr/bin/env python3
"""Offline OPC UA → SECS transport migration; business state is preserved.

SOURCE contains TEL-CVDTIN-08A-NN.json. DESTINATION must be separate.
Repeated runs verify the exact existing output and never overwrite drift.
Gateway spool files and signed recovery envelopes require no rewriting.
"""
import argparse,copy,json,pathlib

def migrate(source,destination):
 source=pathlib.Path(source);destination=pathlib.Path(destination)
 if source.resolve()==destination.resolve():raise ValueError('source and destination must differ')
 destination.mkdir(parents=True,exist_ok=True);destination.chmod(0o700)
 for n in range(1,11):
  id=f'TEL-CVDTIN-08A-{n:02}';name=id+'.json'
  original=json.loads((source/name).read_text())
  if original.get('equipmentId')!=id:raise ValueError('equipment identity mismatch: '+id)
  updated=copy.deepcopy(original);updated['paused']=True
  out=destination/name
  if out.exists():
   if json.loads(out.read_text())!=updated:raise ValueError('existing destination differs: '+id)
  else:out.write_text(json.dumps(updated,ensure_ascii=False));out.chmod(0o600)
 return 10
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('source');p.add_argument('destination');a=p.parse_args()
 print('Verified',migrate(a.source,a.destination),'paused states; original business data unchanged')
