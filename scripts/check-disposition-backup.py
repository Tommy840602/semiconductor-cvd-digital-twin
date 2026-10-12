#!/usr/bin/env python3
"""Read every archived byte and restore both logical DBs into isolated databases."""
import hashlib,json,pathlib,subprocess,tarfile
root=pathlib.Path(__file__).resolve().parents[1];backup=root/'.runtime/backups/cvd-disposition-20260919';compose=root/'.runtime/disposition-compose.json'
assert json.loads(compose.read_text())['name']=='cvd-disposition-test'
dc=['docker','compose','-f',str(compose)];proof={}
for name,m in json.loads((backup/'manifest.json').read_text()).items():
 p=backup/m['archive'];h=hashlib.sha256()
 with p.open('rb') as f:
  for chunk in iter(lambda:f.read(4*1024*1024),b''):h.update(chunk)
 assert h.hexdigest()==m['sha256'],name
 count=total=0
 with tarfile.open(p,'r|gz') as archive:
  for member in archive:
   if not member.isfile():continue
   f=archive.extractfile(member);size=0
   for chunk in iter(lambda:f.read(4*1024*1024),b''):size+=len(chunk)
   assert size==member.size,(name,member.name);count+=1;total+=size
 proof[name]={'sha256':h.hexdigest(),'filesRead':count,'expandedBytesRead':total}
 print('Verified archive',name,count,flush=True)
for service in ['postgres','timescaledb']:
 subprocess.run(dc+['exec','-T',service,'psql','-U','cvd','-d','postgres','-v','ON_ERROR_STOP=1','-c','DROP DATABASE IF EXISTS backup_restore WITH (FORCE)','-c','CREATE DATABASE backup_restore'],check=True,stdout=subprocess.DEVNULL)
 if service=='timescaledb':subprocess.run(dc+['exec','-T',service,'psql','-U','cvd','-d','backup_restore','-v','ON_ERROR_STOP=1','-c','CREATE EXTENSION IF NOT EXISTS timescaledb','-c','SELECT timescaledb_pre_restore()'],check=True,stdout=subprocess.DEVNULL)
 with (backup/(service+'.dump')).open('rb') as f:subprocess.run(dc+['exec','-T',service,'pg_restore','-U','cvd','-d','backup_restore','--exit-on-error','--no-owner'],stdin=f,check=True)
 if service=='timescaledb':subprocess.run(dc+['exec','-T',service,'psql','-U','cvd','-d','backup_restore','-c','SELECT timescaledb_post_restore()'],check=True,stdout=subprocess.DEVNULL)
 tables=['app_users','recovery_requests','recovery_versions','recovery_intents','equipment_snapshot','history','commands'] if service=='postgres' else ['telemetry']
 proof[service+'Restore']={t:int(subprocess.check_output(dc+['exec','-T',service,'psql','-U','cvd','-d','backup_restore','-Atc','SELECT count(*) FROM '+t],text=True)) for t in tables}
 print('Restored',service,proof[service+'Restore'],flush=True)
(root/'.runtime/disposition-backup-proof.json').write_text(json.dumps(proof,indent=2));print('PASS all archives readable and PG/Timescale logical restore',flush=True)
