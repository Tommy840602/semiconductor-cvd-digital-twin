#!/usr/bin/env python3
"""Restore all backup volumes to a new isolated project, Kafka on external disk.
Only creates cvd-secs-restore volumes; refuses existing ones. Does not start apps.
"""
import argparse,hashlib,json,pathlib,subprocess
p=argparse.ArgumentParser();p.add_argument('backup');a=p.parse_args();root=pathlib.Path(__file__).resolve().parents[1];backup=pathlib.Path(a.backup).resolve();project='cvd-secs-restore'
def run(args,**kw):return subprocess.run(args,check=True,cwd=root,**kw)
c=json.loads((root/'.runtime/secs-production-compose.json').read_text());c['name']=project
for name,v in c['networks'].items():v['name']=project+'_'+name
for name,v in c['volumes'].items():v['name']=project+'_'+name
kafka=backup/'restore-kafka'
if kafka.exists():raise SystemExit('Restore target already exists')
kafka.mkdir()
manifest=json.loads((backup/'manifest.json').read_text())
existing=set(subprocess.check_output(['docker','volume','ls','--format','{{.Name}}'],text=True).splitlines())
assert not any(v['name'] in existing for v in c['volumes'].values()),'Restore volume already exists'
for name,d in manifest.items():
 source=backup/d['archive'];h=hashlib.sha256()
 with source.open('rb') as f:
  for b in iter(lambda:f.read(4*1024*1024),b''):h.update(b)
 assert h.hexdigest()==d['sha256'],'Backup checksum mismatch: '+name
 target=str(kafka) if name=='kafka-data' else project+'_'+name
 if name!='kafka-data':run(['docker','volume','create',target],stdout=subprocess.DEVNULL)
 print('Restoring',name,flush=True)
 run(['docker','run','--rm','-v',str(backup)+':/backup:ro','-v',target+':/data','alpine:3.22','tar','xzf','/backup/'+source.name,'-C','/data'])
# ExFAT/macOS may generate AppleDouble metadata while restoring POSIX attributes.
# These are not Kafka segments and were not part of the original volume.
for extra in kafka.rglob('._*'):
 if extra.is_file():extra.unlink()
for name,s in c['services'].items():
 s.pop('ports',None)
 if name in ('frontend','backend'):s['ports']=[{'target':80 if name=='frontend' else 8080,'published':'22000' if name=='frontend' else '22080','host_ip':'127.0.0.1','protocol':'tcp'}]
 for v in s.get('volumes',[]):
  if v['type']=='volume' and v['source']=='kafka-data':v.update(type='bind',source=str(kafka))
c['services']['backend']['environment']['CVD_PUBLIC_ORIGIN']=''
out=root/'.runtime/secs-restore-compose.json';out.write_text(json.dumps(c,indent=2));out.chmod(0o600)
print('Full isolated restore ready; no service started')
