#!/usr/bin/env python3
"""Add KLA MQTT identity to an existing private SECS credential directory; no service restart."""
import argparse,pathlib,secrets,subprocess
p=argparse.ArgumentParser();p.add_argument('--directory',required=True);a=p.parse_args()
d=pathlib.Path(a.directory).resolve()
if not (d/'passwords').is_file() or not (d/'acl').is_file():raise SystemExit('Existing SECS credential directory required')
u='KLA-SIM-01';key=d/u
if not key.exists():key.write_text(secrets.token_urlsafe(32));key.chmod(0o600)
# Encrypt just the new credential, preserving all existing hashes and identities.
tmp=d/'kla-password.tmp';tmp.write_text(u+':'+key.read_text().strip()+'\n');tmp.chmod(0o600)
try:
 subprocess.run(['docker','run','--rm','-v',str(d)+':/secrets','eclipse-mosquitto:2.0','mosquitto_passwd','-U','/secrets/kla-password.tmp'],check=True)
 lines=[x for x in (d/'passwords').read_text().splitlines() if not x.startswith(u+':')]
 (d/'passwords').write_text('\n'.join(lines)+'\n'+tmp.read_text());(d/'passwords').chmod(0o644)
finally:tmp.unlink(missing_ok=True)
acl=(d/'acl').read_text()
if '\nuser '+u+'\n' not in acl:
 acl+='\nuser '+u+'\n'
 for k in ['NBIRTH','NDEATH','NDATA','DBIRTH','DDEATH','DDATA']:acl+='topic write spBv1.0/cvd-fab/'+k+'/edge-'+u+('/'+u if k.startswith('D') else '')+'\n'
 for k in ['NCMD','DCMD']:acl+='topic read spBv1.0/cvd-fab/'+k+'/edge-'+u+('/'+u if k.startswith('D') else '')+'\n'
 acl+='topic read spBv1.0/STATE/cvd-data-platform\n';(d/'acl').write_text(acl)
print('KLA identity prepared; reload MQTT configuration before starting the gateway.')
