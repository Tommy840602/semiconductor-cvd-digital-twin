"""Record measured demo size and query latency; no production writes."""
import datetime,json,subprocess,time,urllib.parse,urllib.request,statistics
import os
os.environ.setdefault('CVD_URL','http://localhost:19080')
os.environ.setdefault('CVD_PASSWORD_FILE','.runtime/secrets/test-leader-password')
from scenarios import request
DC=['docker','compose','-f','.runtime/process-compose.json']
q="SELECT json_build_object('rows',count(*),'avgBytes',round(avg(octet_length(body::text))),'maxBytes',max(octet_length(body::text))) FROM telemetry"
size=json.loads(subprocess.check_output(DC+['exec','-T','timescaledb','psql','-U','cvd','-d','cvd','-Atc',q],text=True))
now=datetime.datetime.now(datetime.timezone.utc);params={'pmId':'PM1','metric':'thickness','from':(now-datetime.timedelta(days=30)).isoformat(),'to':now.isoformat()}
url='http://localhost:19080/api/telemetry?'+urllib.parse.urlencode(params);durations=[]
for _ in range(10):
 start=time.perf_counter()
 body=request('/telemetry?'+urllib.parse.urlencode(params))
 durations.append((time.perf_counter()-start)*1000)
print(json.dumps({'measuredAt':now.isoformat(),'dataset':size,'rangeDays':30,'resolutionSeconds':body['resolutionSeconds'],'points':len(body['points']),'requests':10,'medianMs':round(statistics.median(durations),2),'maxMs':round(max(durations),2),'note':'既有驗收樣本；非30天全量負載測試'},ensure_ascii=False,indent=2))
