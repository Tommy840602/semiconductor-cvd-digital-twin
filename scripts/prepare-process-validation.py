#!/usr/bin/env python3
"""Generate isolated validation Compose; does not start or alter services."""
import json,os,pathlib,subprocess,secrets
root=pathlib.Path(__file__).resolve().parents[1]
config=json.loads(subprocess.check_output(['docker','compose','config','--format','json'],cwd=root,text=True))
project='cvd-process-test';prefix=os.getenv('IMAGE_PREFIX','cvd-process-candidate')
config['name']=project
for name,service in config['services'].items():
 if 'build' in service:
  service.pop('build');service['image']=prefix+'-'+name
 service.pop('ports',None)
 if name in ('backend','frontend'):
  service['ports']=[{'target':8080 if name=='backend' else 80,'published':'19080' if name=='backend' else '19000','host_ip':'127.0.0.1','protocol':'tcp'}]
for section in ('networks','volumes'):
 for name,item in config.get(section,{}).items():item['name']=project+'_'+name
secret=root/'.runtime/secrets/test-leader-password';secret.parent.mkdir(parents=True,exist_ok=True)
if not secret.exists():secret.write_text(secrets.token_urlsafe(24));secret.chmod(0o600)
config['secrets']['leader_password']['file']=str(secret)
config['services']['backend']['environment']['CVD_PUBLIC_ORIGIN']=''
output=root/'.runtime/process-compose.json';output.parent.mkdir(exist_ok=True)
output.write_text(json.dumps(config,indent=2)+'\n')
print(output)
