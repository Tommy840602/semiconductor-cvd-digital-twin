"""Same-tick short recipe phase boundaries keep their source sequence order."""
import datetime,json,subprocess,urllib.parse,urllib.request
import os
os.environ.setdefault('CVD_URL','http://localhost:19080')
os.environ.setdefault('CVD_PASSWORD_FILE','.runtime/secrets/test-leader-password')
from scenarios import request
DC=['docker','compose','-f','.runtime/process-compose.json']
def sql(s):return subprocess.check_output(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-At'],input=s,text=True)
t=(datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=1)).replace(microsecond=0)
try:
 for i,phase in enumerate(['purge','complete']):
  e={'schemaVersion':1,'eventId':'order-fixture-'+('z' if i==0 else 'a'),'sequence':100+i,'sourceTime':t.isoformat(),'kind':'process-phase-changed','waferId':'ORDER-W01','payload':{'pm':'PM4','phase':phase,'processRunId':'order-run'}}
  sql("INSERT INTO history(id,time,wafer_id,kind,body) VALUES ('%s','%s','ORDER-W01','process-phase-changed','%s');"%(e['eventId'],t.isoformat(),json.dumps(e)))
 q=urllib.parse.urlencode({'pmId':'PM4','waferId':'ORDER-W01','from':t.isoformat(),'to':(t+datetime.timedelta(seconds=1)).isoformat()})
 events=request('/process-history?'+q)
 assert [e['payload']['phase'] for e in events]==['purge','complete'],events
 print('PASS same-tick phases ordered by sequence, independent of event UUID')
finally:sql("DELETE FROM history WHERE id IN ('order-fixture-z','order-fixture-a');")
