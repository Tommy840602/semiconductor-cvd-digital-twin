#!/usr/bin/env python3
"""Isolated PHM stack, using fresh data and dedicated MQTT credentials."""
import json,pathlib,subprocess,secrets,copy
root=pathlib.Path(__file__).resolve().parents[1];private=root/'.runtime/phm-test-secrets';private.mkdir(exist_ok=True);private.chmod(0o700)
def key(n):
 p=private/n
 if not p.exists():p.write_text(secrets.token_urlsafe(32));p.chmod(0o600)
 return str(p)
base=json.loads(subprocess.check_output(['docker','compose','config','--format','json'],cwd=root))
names=['postgres','timescaledb','redis','kafka','mqtt','backend','bridge','simulator','gateway','metrology','metrology-gateway']
c={'name':'cvd-phm-validation','services':{n:copy.deepcopy(base['services'][n]) for n in names},'volumes':{},'secrets':{}}
for n,s in c['services'].items():
 for f in ['build','ports','networks']:s.pop(f,None)
 for v in s.get('volumes',[]):
  if v.get('type')=='volume':c['volumes'][v['source']]={}
 for sec in s.get('secrets',[]):
  source=sec['source'];c['secrets'][source]={'file':key(source)}
c['services']['backend']['image']='cvd-phm-backend';c['services']['backend']['ports']=[{'target':8080,'published':'19080','host_ip':'127.0.0.1'}];c['services']['backend']['environment']['EMAIL_ENABLED']='false';c['services']['backend']['environment']['CVD_PUBLIC_ORIGIN']=''
c['services']['kafka']['healthcheck']['timeout']='60s'
c['services']['kafka']['healthcheck']['start_period']='90s'
c['services']['simulator']['image']='cvd-phm-simulator'
fixture=root/'.runtime/phm-fixture';fixture.mkdir(exist_ok=True)
c['services']['simulator']['volumes']=[{'type':'bind','source':str(fixture),'target':'/data'}]
users={'health':'mqtt_health','cvd-bridge':'mqtt_bridge','TEL-CVDTIN-08A-01':'TEL-CVDTIN-08A-01','KLA-SIM-01':'kla_mqtt'}
(private/'passwords').write_text(''.join(u+':'+pathlib.Path(key(k)).read_text()+'\n' for u,k in users.items()))
subprocess.run(['docker','run','--rm','-v',str(private)+':/secrets','eclipse-mosquitto:2.0','mosquitto_passwd','-U','/secrets/passwords'],check=True)
(private/'passwords').chmod(0o644)
acl='user health\ntopic write health/mqtt\nuser cvd-bridge\ntopic readwrite spBv1.0/#\n'
for eq in ['TEL-CVDTIN-08A-01','KLA-SIM-01']:acl+='user '+eq+'\ntopic readwrite spBv1.0/cvd-fab/+/edge-'+eq+'/#\ntopic read spBv1.0/STATE/cvd-data-platform\n'
(private/'acl').write_text(acl);(private/'mosquitto.conf').write_text('listener 1883\nallow_anonymous false\npassword_file /mosquitto/config/passwords\nacl_file /mosquitto/config/acl\npersistence true\npersistence_location /mosquitto/data/\n')
c['services']['mqtt']['volumes']=[{'type':'bind','source':str(private/n),'target':'/mosquitto/config/'+n,'read_only':True} for n in ['passwords','acl','mosquitto.conf']]+[{'type':'volume','source':'mqtt-data','target':'/mosquitto/data'}]
(root/'.runtime/phm-validation.json').write_text(json.dumps(c,indent=2));print('Prepared isolated cvd-phm-validation')
