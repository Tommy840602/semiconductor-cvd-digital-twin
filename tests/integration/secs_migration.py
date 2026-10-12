"""Offline migration invariants, independent of Docker or production."""
import importlib.util,json,pathlib,tempfile
spec=importlib.util.spec_from_file_location('migration','scripts/secs-state-migration.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
with tempfile.TemporaryDirectory() as root:
 root=pathlib.Path(root);src=root/'source';dest=root/'destination';src.mkdir()
 for n in range(1,11):
  id=f'TEL-CVDTIN-08A-{n:02}';state={'equipmentId':id,'paused':False,'epoch':'original-epoch','sequence':100+n,'recipeVersion':id+'-r3','lots':[{'wafer':{'location':'PM1','thickness':2.125}}],'recoveries':{'original-execution':{'signature':'do-not-rewrite','status':'executing'}},'commands':{'original-command':{'status':'accepted'}},'events':[{'eventId':'original-id'}]}
  (src/(id+'.json')).write_text(json.dumps(state))
 assert m.migrate(src,dest)==10;assert m.migrate(src,dest)==10
 for p in src.iterdir():
  old=json.loads(p.read_text());new=json.loads((dest/p.name).read_text());assert new.pop('paused') is True;assert old.pop('paused') is False;assert old==new
 # Refuse to overwrite a destination that was changed after migration.
 p=next(dest.iterdir());s=json.loads(p.read_text());s['sequence']+=1;p.write_text(json.dumps(s))
 try:m.migrate(src,dest);raise AssertionError('drift accepted')
 except ValueError:pass
print('PASS: two migrations preserve wafer, signature, command and event identity; drift rejected')
