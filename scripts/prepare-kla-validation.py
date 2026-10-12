#!/usr/bin/env python3
"""Generate an isolated KLA verification project; never changes production services."""
import json,pathlib,subprocess,secrets
root=pathlib.Path(__file__).resolve().parents[1];private=root/'.runtime/kla-test-secrets';private.mkdir(exist_ok=True);private.chmod(0o700)
def key(name):
 p=private/name
 if not p.exists():p.write_text(secrets.token_urlsafe(32));p.chmod(0o600)
 return str(p)
base=json.loads(subprocess.check_output(['docker','compose','config','--format','json'],cwd=root));names=['postgres','timescaledb','redis','kafka','mqtt','backend','bridge'];c={'name':'cvd-kla-validation','services':{n:base['services'][n] for n in names},'volumes':{},'secrets':{}}
for n,s in c['services'].items():
 s.pop('build',None);s.pop('ports',None);s.pop('networks',None);s.pop('restart',None)
 for v in s.get('volumes',[]):
  if v.get('type')=='volume':c['volumes'][v['source']]={}
for name in ['leader_password','recovery_key','control_key','mqtt_bridge','mqtt_health','kla_mqtt']:c['secrets'][name]={'file':key(name)}
b=c['services']['backend'];b['image']='cvd-kla-backend';b['ports']=[{'target':8080,'published':'19080','host_ip':'127.0.0.1'}];b['environment']['METROLOGY_ENABLED']='true';b['environment']['EMAIL_ENABLED']='false';b['environment']['CVD_PUBLIC_ORIGIN']=''
c['services']['bridge']['image']='cvd-kla-bridge'
users={'health':'mqtt_health','cvd-bridge':'mqtt_bridge','KLA-SIM-01':'kla_mqtt'}
(private/'passwords').write_text(''.join(u+':'+pathlib.Path(key(k)).read_text()+'\n' for u,k in users.items()))
subprocess.run(['docker','run','--rm','-v',str(private)+':/secrets','eclipse-mosquitto:2.0','mosquitto_passwd','-U','/secrets/passwords'],check=True)
(private/'passwords').chmod(0o644)
acl='user health\ntopic write health/mqtt\nuser cvd-bridge\ntopic readwrite spBv1.0/#\nuser KLA-SIM-01\ntopic readwrite spBv1.0/cvd-fab/+/edge-KLA-SIM-01/#\ntopic read spBv1.0/STATE/cvd-data-platform\n'
(private/'acl').write_text(acl);(private/'mosquitto.conf').write_text('listener 1883\nallow_anonymous false\npassword_file /mosquitto/config/passwords\nacl_file /mosquitto/config/acl\npersistence true\npersistence_location /mosquitto/data/\n')
c['services']['mqtt']['volumes']=[{'type':'bind','source':str(private/n),'target':'/mosquitto/config/'+n,'read_only':True} for n in ['passwords','acl','mosquitto.conf']]+[{'type':'volume','source':'mqtt-data','target':'/mosquitto/data'}]
addon=json.loads(subprocess.check_output(['docker','compose','-f','compose.yaml','-f','compose.secs.yaml','-f','compose.metrology.yaml','config','--format','json'],cwd=root))
for n in ['metrology','metrology-gateway']:
 s=addon['services'][n];s.pop('build',None);s.pop('networks',None);s.pop('restart',None)
 if n=='metrology-gateway':s['image']='cvd-kla-gateway'
 c['services'][n]=s
 for v in s.get('volumes',[]):
  if v.get('type')=='volume':c['volumes'][v['source']]={}
(root/'.runtime/kla-validation.json').write_text(json.dumps(c,indent=2));print('Prepared isolated cvd-kla-validation project')
