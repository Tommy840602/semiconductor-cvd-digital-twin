#!/usr/bin/env python3
"""Prepare SECS isolated deployment. Does not operate production containers."""
import json,pathlib,subprocess,secrets,argparse
p=argparse.ArgumentParser();p.add_argument("--production",action="store_true",help="Prepare production configuration only; never starts or stops services")
a=p.parse_args();project="cvd-twin" if a.production else "cvd-secs-test"
root=pathlib.Path(__file__).resolve().parents[1];private=root/('.runtime/secrets/secs-production' if a.production else '.runtime/secrets/secs');private.mkdir(parents=True,exist_ok=True);private.chmod(0o700)
def secret(name):
 p=private/name
 if not p.exists():p.write_text(secrets.token_urlsafe(32));p.chmod(0o600)
 return p
users=['cvd-bridge','health']+[f'TEL-CVDTIN-08A-{n:02}' for n in range(1,11)]
passwd=private/'passwords';passwd.write_text(''.join(u+':'+secret(u).read_text()+'\n' for u in users));passwd.chmod(0o600)
subprocess.run(['docker','run','--rm','-v',str(private)+':/secrets','eclipse-mosquitto:2.0','mosquitto_passwd','-U','/secrets/passwords'],check=True)
# Mosquitto runs as uid 1883; directory is private on the host, mount itself needs readable files.
passwd.chmod(0o644)
acl='user health\ntopic write health/mqtt\n\nuser cvd-bridge\ntopic read spBv1.0/cvd-fab/#\ntopic write spBv1.0/cvd-fab/NCMD/#\ntopic write spBv1.0/cvd-fab/DCMD/#\ntopic readwrite spBv1.0/STATE/cvd-data-platform\n'
for u in users[2:]:
 acl+='\nuser '+u+'\n'
 for k in ['NBIRTH','NDEATH','NDATA','DBIRTH','DDEATH','DDATA']:acl+='topic write spBv1.0/cvd-fab/'+k+'/edge-'+u+('/'+u if k.startswith('D') else '')+'\n'
 for k in ['NCMD','DCMD']:acl+='topic read spBv1.0/cvd-fab/'+k+'/edge-'+u+('/'+u if k.startswith('D') else '')+'\n'
 acl+='topic read spBv1.0/STATE/cvd-data-platform\n'
(private/'acl').write_text(acl)
(private/'mosquitto.conf').write_text('listener 1883\nallow_anonymous false\npassword_file /mosquitto/config/passwords\nacl_file /mosquitto/config/acl\npersistence true\npersistence_location /mosquitto/data/\nmax_inflight_messages 1000\n')
c=json.loads(subprocess.check_output(['docker','compose','-f','compose.yaml','config','--format','json'],cwd=root));c['name']=project
for section in ['networks','volumes']:
 for name,v in c[section].items():v['name']=project+'_'+name
c['volumes']['bridge-data']={'name':project+'_bridge-data'}
c['secrets']['control_key']={'file':str(secret('control-key'))}
if not a.production:c['secrets']['leader_password']['file']=str(secret('leader-password'))
for name,s in c['services'].items():
 if 'build' in s:s.pop('build');s['image']='cvd-secs-'+('simulator' if name.startswith('simulator') else 'gateway' if name.startswith('gateway') else name)
 if not a.production:s.pop('ports',None)
 if not a.production and name in ['frontend','backend']:s['ports']=[{'target':80 if name=='frontend' else 8080,'published':'21000' if name=='frontend' else '21080','host_ip':'127.0.0.1','protocol':'tcp'}]
 if name=='backend' or name.startswith(('simulator','gateway')):
  s.setdefault('environment',{})['CVD_PROTOCOL']='secs';s['secrets']=s.get('secrets',[])+[{'source':'control_key','target':'control_key'}];s['restart']='unless-stopped'
 if name.startswith('gateway'):
  u=s['environment']['EQUIPMENT_ID'];endpoint=s['environment'].pop('OPC_ENDPOINT');s['environment']['HSMS_ENDPOINT']=endpoint.removeprefix('opc.tcp://').replace(':4840',':5000');c['secrets'][u]={'file':str(secret(u))};s['secrets']+=[{'source':u,'target':'mqtt_password'}]
if not a.production:c['services']['backend']['environment']['CVD_PUBLIC_ORIGIN']=''
c['services']['mqtt']['volumes']=[{'type':'bind','source':str(private/name),'target':'/mosquitto/config/'+name,'read_only':True} for name in ['mosquitto.conf','passwords','acl']]+[{'type':'volume','source':'mqtt-data','target':'/mosquitto/data'}]
c['secrets']['mqtt_health']={'file':str(secret('health'))};c['services']['mqtt']['secrets']=[{'source':'mqtt_health','target':'mqtt_health'}]
c['services']['mqtt']['healthcheck']['test']=['CMD-SHELL','mosquitto_pub -h localhost -u health -P "$(cat /run/secrets/mqtt_health)" -t health/mqtt -m ok']
c['secrets']['mqtt_bridge']={'file':str(secret('cvd-bridge'))}
c['services']['bridge']={'image':'cvd-secs-bridge','environment':{'DATA_DIR':'/data','MQTT_URL':'tcp://mqtt:1883','KAFKA_BOOTSTRAP':'kafka:9092'},'volumes':[{'type':'volume','source':'bridge-data','target':'/data'}],'secrets':[{'source':'control_key','target':'control_key'},{'source':'mqtt_bridge','target':'mqtt_password'}],'depends_on':{'mqtt':{'condition':'service_healthy'},'kafka':{'condition':'service_healthy'}},'restart':'unless-stopped','healthcheck':{'test':['CMD-SHELL','wget -qO- http://localhost:8083/health'],'interval':'3s','timeout':'3s','retries':30}}
out=root/('.runtime/secs-production-compose.json' if a.production else '.runtime/secs-compose.json');out.write_text(json.dumps(c,indent=2));out.chmod(0o600)
print('Prepared '+project+' configuration only; no running services changed')

if a.production:
 overlay={'services':{},'secrets':{},'volumes':{'bridge-data':{}}}
 for name,service in c['services'].items():
  if name.startswith(('simulator','gateway')) or name=='backend':
   item={'image':service['image'],'restart':'unless-stopped','environment':{'CVD_PROTOCOL':'secs'},'secrets':service['secrets']}
   if name.startswith('gateway'):item['environment'].update(HSMS_ENDPOINT=service['environment']['HSMS_ENDPOINT'],OPC_ENDPOINT=None)
   overlay['services'][name]=item
  elif name=='frontend':overlay['services'][name]={'image':service['image']}
  elif name in ('mqtt','bridge'):
   overlay['services'][name]={k:service[k] for k in (['volumes','secrets','healthcheck'] if name=='mqtt' else service.keys())}
   if name=='bridge':overlay['services'][name]['build']={'context':'.','dockerfile':'bridge/Dockerfile'}
 for name,value in c['secrets'].items():
  if name not in ('leader_password','recovery_key'):overlay['secrets'][name]=value
 # Keep the checked-in overlay portable; secrets remain generated and ignored.
 def relative(value):
  if isinstance(value,dict):return {k:relative(v) for k,v in value.items()}
  if isinstance(value,list):return [relative(v) for v in value]
  if isinstance(value,str) and value.startswith(str(root)+'/'):return './'+value[len(str(root))+1:]
  return value
 (root/'compose.secs.yaml').write_text(json.dumps(relative(overlay),indent=2)+'\n')
