#!/usr/bin/env python3
"""Reset only disposable fleet-test fixtures; preserves the restored migration databases."""
import subprocess,json
DC=['docker','compose','-f','.runtime/fleet-compose.json'];c=json.loads(subprocess.check_output(DC+['config','--format','json'],text=True));assert c['name']=='cvd-fleet-test'
apps=[n for n in c['services'] if n.startswith(('simulator','gateway')) or n in ('backend','frontend','mqtt','kafka')]
subprocess.run(DC+['stop','-t','3',*apps],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
subprocess.run(DC+['rm','-f','mqtt','kafka'],check=True,stdout=subprocess.DEVNULL)
subprocess.run(['docker','volume','rm','cvd-fleet-test_mqtt-data','cvd-fleet-test_kafka-data'],check=True,stdout=subprocess.DEVNULL)
subprocess.run(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-c','TRUNCATE inbox,history,commands,equipment_snapshot,dead_letter,recovery_requests,recovery_versions,recovery_intents,recovery_events,recipe_heads,recipe_versions CASCADE'],check=True,stdout=subprocess.DEVNULL)
subprocess.run(DC+['exec','-T','timescaledb','psql','-U','cvd','-d','cvd','-c','TRUNCATE telemetry'],check=True,stdout=subprocess.DEVNULL)
volumes=[]
for name,s in c['services'].items():
 if name.startswith(('simulator','gateway')):
  for v in s['volumes']:
   if v['type']=='volume':volumes.extend(['-v',c['volumes'][v['source']]['name']+':/fixtures/'+v['source']])
subprocess.run(['docker','run','--rm',*volumes,'alpine:3.22','sh','-c','rm -f /fixtures/*/state.json /fixtures/*/spool.json'],check=True)
print('Reset isolated fleet fixtures; users, production and fleet_restore databases preserved')
