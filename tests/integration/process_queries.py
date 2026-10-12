"""Deterministic SQL/API regression. Isolated cvd-process-test project only."""
import copy,datetime,json,subprocess,urllib.request,urllib.error,urllib.parse,time
import os
os.environ.setdefault('CVD_URL','http://localhost:19080')
os.environ.setdefault('CVD_PASSWORD_FILE','.runtime/secrets/test-leader-password')
from scenarios import request
BASE='http://localhost:19080/api'
def sql(statement):
 return subprocess.check_output(['docker','compose','-f','.runtime/process-compose.json','exec','-T','timescaledb','psql','-U','cvd','-d','cvd','-At'],input=statement,text=True).strip()
def get(**kw):
 return request('/telemetry?'+urllib.parse.urlencode(kw))
def stamp(t):return t.isoformat().replace('+00:00','Z')
now=datetime.datetime.now(datetime.timezone.utc)
t=(now-datetime.timedelta(days=1)).replace(hour=0,minute=0,second=0,microsecond=0)
fixture=json.load(open('contracts/process-event.json'))
statements=[]
for i,value in enumerate([None,2,4,8]):
 e=copy.deepcopy(fixture);e['eventId']='query-fixture-'+str(i);e['sourceTime']=stamp(t+datetime.timedelta(seconds=[0,1,3,4][i]));p=e['payload']['chambers'][0];p['waferId']='QUERY-FIXTURE-W01';p['processRunId']='run-a' if i<3 else 'run-b';p['processTelemetry']['thicknessNm']=value
 if value is None:p.pop('processTelemetry');p.pop('gasChannels')
 statements.append("INSERT INTO telemetry(time,id,body) VALUES ('%s','%s','%s') ON CONFLICT DO NOTHING;"%(e['sourceTime'],e['eventId'],json.dumps(e).replace("'","''")))
try:
 sql('\n'.join(statements))
 q=dict(metric='thickness',pmId='PM1',waferId='QUERY-FIXTURE-W01',**{'from':stamp(t),'to':stamp(t+datetime.timedelta(seconds=5))})
 points=get(**q)['points'];assert [p['actual'] for p in points]==[None,2,4,8],points
 assert [p['sampleCount'] for p in points]==[0,1,1,1]
 assert len(points)==4 and points[2]['time'].endswith('03Z'),points
 q['to']=stamp(t+datetime.timedelta(seconds=4));assert len(get(**q)['points'])==3
 q['to']=stamp(t+datetime.timedelta(hours=2));r=get(**q);assert r['resolutionSeconds']==60
 a,b=r['points'];assert (a['actual'],a['min'],a['max'],a['sampleCount'])==(3,2,4,2),a;assert b['actual']==8 and b['processRunId']=='run-b'
 q['processRunId']='run-a';assert len(get(**q)['points'])==1
 q['waferId']='missing';assert get(**q)['points']==[]
 for kw in [dict(metric='bad',pmId='PM1'),dict(metric='power',llId='LL1'),dict(metric='pressure',llId='LL4'),dict(metric='pressure',llId='LL1',pmId='PM1')]:
  try:get(**kw);raise AssertionError('invalid accepted')
  except urllib.error.HTTPError as e:assert e.code==400
 assert get(metric='thickness',pmId='PM1',**{'from':stamp(now-datetime.timedelta(days=40)),'to':stamp(now-datetime.timedelta(days=31))})['points']==[]
 # 2000-point guard must reject rather than silently truncate.
 sql("INSERT INTO telemetry(time,id,body) SELECT '"+stamp(t)+"'::timestamptz + i*interval '1 second','query-fixture-many-'||i,jsonb_build_object('payload',jsonb_build_object('chambers',jsonb_build_array(jsonb_build_object('id','PM2','temperature',i)))) FROM generate_series(0,2000) i;")
 try:get(metric='temperature',pmId='PM2',**{'from':stamp(t),'to':stamp(t+datetime.timedelta(seconds=2100))});raise AssertionError('truncation accepted')
 except urllib.error.HTTPError as e:assert e.code==422
 print('PASS raw/null, [from,to), minute avg/min/max/count, run/wafer isolation, invalid metrics, 30-day cutoff')
finally:sql("DELETE FROM telemetry WHERE id LIKE 'query-fixture-%';")
