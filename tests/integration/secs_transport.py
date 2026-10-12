"""Destructive outage tests ONLY on the named isolated SECS stack."""
from fleet import *
import subprocess,datetime
DC=['docker','compose','-f','.runtime/secs-compose.json']
assert ':21000/' in BASE

def dc(*args):return subprocess.check_output(DC+list(args),text=True,stderr=subprocess.STDOUT).strip()
def sql(q):return dc('exec','-T','timescaledb','psql','-U','cvd','-d','cvd','-Atc',q)
def metrics(service,port):return json.loads(dc('exec','-T',service,'wget','-qO-',f'http://localhost:{port}/metrics'))
def good(c):
 f=c.call('/equipment');now=time.time()
 return all(m.get('communicationQuality')=='good' and now-datetime.datetime.fromisoformat(m['sourceTime'].replace('Z','+00:00')).timestamp()<5 for m in f)
def main():
 c=Client();wait(lambda:good(c));start=sql('select now()')
 result={}
 # v5 authorization uses PUBACK reason code (MQTT 3.1.1 can silently discard).
 def pub(user,topic):
  p=subprocess.run(['docker','run','--rm','--network','cvd-secs-test_default','-v',str(pathlib.Path('.runtime/secrets/secs').resolve())+':/secrets:ro','eclipse-mosquitto:2.0','sh','-c','exec mosquitto_pub -V mqttv5 -q 1 -h mqtt -u "$1" -P "$(cat /secrets/"$1")" -t "$2" -m invalid','sh',user,topic],capture_output=True,text=True)
  return p.returncode,p.stderr
 for label,u,t in [('crossTool',IDS[0],f'spBv1.0/cvd-fab/DDATA/edge-{IDS[1]}/{IDS[1]}'),('gatewayCommand',IDS[0],f'spBv1.0/cvd-fab/DCMD/edge-{IDS[0]}/{IDS[0]}'),('bridgeData','cvd-bridge',f'spBv1.0/cvd-fab/DDATA/edge-{IDS[0]}/{IDS[0]}')]:
  code,error=pub(u,t);assert 'not authorized' in error.lower(),(label,code,error);result[label+'Denied']=True
 print('Broker ACL negative tests passed',flush=True)
 # The engine continues sampling while Kafka cannot acknowledge delivery.
 dc('stop','kafka')
 try:
  time.sleep(20);m=metrics('bridge',8083);assert m['pendingRecords']>50,m;result['kafkaOutagePending']=m['pendingRecords']
 finally:dc('start','kafka')
 wait(lambda:good(c),120);wait(lambda:metrics('bridge',8083)['pendingRecords']<40,120)
 # Crash after an ACK-capable durable spool has accumulated; restart must replay.
 dc('stop','mqtt')
 try:time.sleep(12);dc('kill','bridge')
 finally:dc('start','mqtt');dc('start','bridge')
 wait(lambda:good(c),120);result['brokerAndBridgeRestartRecovered']=True
 # Single gateway death must not mark a different tool offline.
 before=c.tool(IDS[1])['sourceTime'];dc('stop','gateway')
 try:
  wait(lambda:c.tool(IDS[0]).get('communicationQuality')!='good',30)
  wait(lambda:c.tool(IDS[1])['sourceTime']>before and c.tool(IDS[1]).get('communicationQuality')=='good',30)
 finally:dc('start','gateway')
 wait(lambda:good(c),90);time.sleep(8)
 # One durable telemetry record each source second; replay is deduplicated.
 end=sql('select now()')
 def drained():
  counts=sql("select body->>'equipmentId',count(*),count(distinct id),max(extract(epoch from time))-min(extract(epoch from time)) from telemetry where time>='"+start+"' and time<='"+end+"' group by 1 order by 1")
  if len(counts.splitlines())!=10:return None
  for row in counts.splitlines():
   id,n,unique,span=row.split('|');assert n==unique
   if int(n)<float(span)-3:return None
  return counts
 counts=wait(drained,60)
 result['samplingContinuity']=counts;result['historicalReplayRecords']=int(sql("select count(*) from telemetry where time>='"+start+"' and body->>'historical'='true'"));assert result['historicalReplayRecords']>0
 result['finalBridgeMetrics']=metrics('bridge',8083)
 pathlib.Path('.runtime/secs-transport.json').write_text(json.dumps(result,indent=2));print('PASS: ACL, Kafka/Broker outage, bridge crash, single gateway isolation, historical replay and continuity')
if __name__=='__main__':main()
