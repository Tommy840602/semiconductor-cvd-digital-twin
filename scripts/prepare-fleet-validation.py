#!/usr/bin/env python3
"""Create an isolated fleet validation deployment without changing production."""
import json,pathlib,subprocess,secrets
root=pathlib.Path(__file__).resolve().parents[1]
c=json.loads(subprocess.check_output(['docker','compose','config','--format','json'],cwd=root,text=True));project='cvd-fleet-test';c['name']=project
for name,s in c['services'].items():
 if 'build' in s:
  s.pop('build');s['image']='cvd-fleet-'+('simulator' if name.startswith('simulator') else 'gateway' if name.startswith('gateway') else name)
 s.pop('ports',None)
 if name in ('backend','frontend'):s['ports']=[{'target':8080 if name=='backend' else 80,'published':'20080' if name=='backend' else '20000','host_ip':'127.0.0.1','protocol':'tcp'}]
for section in ('networks','volumes'):
 for name,item in c.get(section,{}).items():item['name']=project+'_'+name
secret=root/'.runtime/secrets/fleet-test-password';secret.parent.mkdir(exist_ok=True)
if not secret.exists():secret.write_text(secrets.token_urlsafe(24));secret.chmod(0o600)
c['secrets']['leader_password']['file']=str(secret);c['services']['backend']['environment']['CVD_PUBLIC_ORIGIN']=''
(root/'.runtime/fleet-compose.json').write_text(json.dumps(c,indent=2)+'\n')
print('Prepared isolated cvd-fleet-test configuration')
