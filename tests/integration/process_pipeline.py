"""Compare live OPC UA gateway MQTT payload with persisted Kafka consumer output."""
import json,subprocess,time
DC=['docker','compose','-f','.runtime/process-compose.json']
def dc(*a,input=None):return subprocess.check_output(DC+list(a),input=input,text=True)
def sql(service,q):return dc('exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc',q).strip()
for _ in range(60):
 raw=dc('exec','-T','mqtt','mosquitto_sub','-t','cvd/CVD-01/telemetry','-C','1','-W','5');e=json.loads(raw)
 if any(p.get('processTelemetry') for p in e['payload']['chambers']):break
else:raise AssertionError('no active process telemetry')
assert all('pumpState' in l and 'valveOpeningPct' in l for l in e['payload']['locks'])
for _ in range(30):
 body=sql('timescaledb',"SELECT body FROM telemetry WHERE id='"+e['eventId']+"'")
 if body:break
 time.sleep(1)
stored=json.loads(body)
assert stored.pop("ingestedAt",None)
assert stored==e, {k:(e.get(k),stored.get(k)) for k in set(e)|set(stored) if e.get(k)!=stored.get(k)}
# QoS replay through MQTT and Kafka inbox remains one stored event.
for _ in range(2):dc('exec','-T','mqtt','mosquitto_pub','-q','1','-t','cvd/CVD-01/telemetry','-s',input=raw)
time.sleep(3)
assert sql('timescaledb',"SELECT count(*) FROM telemetry WHERE id='"+e['eventId']+"'")=='1'
print('PASS live OPC UA→gateway→MQTT→Kafka→Timescale equality and replay dedup; payload bytes',len(raw.encode()))

fixture=json.load(open('contracts/process-event.json'))
raw=json.dumps(fixture)
dc('exec','-T','mqtt','mosquitto_pub','-q','1','-t','cvd/CVD-01/telemetry','-s',input=raw)
for _ in range(30):
 body=sql('timescaledb',"SELECT body FROM telemetry WHERE id='fixture-process-1'")
 if body:break
 time.sleep(1)
stored=json.loads(body);stored.pop('ingestedAt');assert stored==fixture
print('PASS golden contract fixture MQTT/Kafka/storage equality')
body=sql('postgres',"SELECT body FROM history WHERE kind='process-result' ORDER BY time LIMIT 1")
result=json.loads(body)
for _ in range(2):dc('exec','-T','mqtt','mosquitto_pub','-q','1','-t','cvd/CVD-01/events','-s',input=body)
time.sleep(3)
assert sql('postgres',"SELECT count(*) FROM history WHERE id='"+result['eventId']+"'")=='1'
print('PASS process-result replay dedup')
