#!/usr/bin/env python3
"""End-to-end synthetic wafers in cvd-kla-validation ONLY."""
import datetime,json,pathlib,subprocess,time,urllib.request,http.cookiejar
root=pathlib.Path(__file__).resolve().parents[1];dc=['docker','compose','-f',str(root/'.runtime/kla-validation.json')];base='http://127.0.0.1:19080/api';op=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()));csrf=''
def api(path,data=None):
 r=urllib.request.Request(base+path,data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json','X-CSRF-Token':csrf});return json.load(op.open(r,timeout=10))
for _ in range(60):
 try:csrf=api('/auth/csrf')['token'];break
 except Exception:time.sleep(1)
csrf=api('/auth/login',{'username':'cvd.leader','password':(root/'.runtime/kla-test-secrets/leader_password').read_text().strip()})['csrf']
stamp=datetime.datetime.now(datetime.timezone.utc).isoformat();tag=str(time.time_ns());events=[]
for i in range(1,11):
 eq=f'TEL-CVDTIN-08A-{i:02}';w=f'kla-fixture-{tag}-{i}';run=w+'/run';source={'recipeVersion':'fixture-v1','processRunId':run,'sampleThicknessNm':[10,10.1,9.9,10.2,9.8],'thicknessNm':10,'targetThicknessNm':10,'thicknessTolerancePct':10,'uniformityLimitPct':5}
 common={'equipmentId':eq,'waferId':w,'lotId':'kla-fixture-'+tag,'sourceTime':stamp,'sourceEpoch':tag,'schemaVersion':1}
 done={**common,'eventId':w+'-complete','kind':'process-result','sequence':1,'payload':{'processRunId':run,'pm':'PM1','slot':1,'purpose':'product','complete':True,'result':source}}
 ret={**common,'eventId':w+'-return','kind':'movement-complete','sequence':2,'payload':{'processRunId':run,'from':'LL1','to':'LP1','robot':'ATM','result':'success','slot':1,'purpose':'product'}}
 events.extend([ret,done] if i%2==0 else [done,ret])
raw='\n'.join(json.dumps(e) for e in events+events)+'\n'
subprocess.run(dc+['exec','-T','kafka','/opt/kafka/bin/kafka-console-producer.sh','--bootstrap-server','localhost:9092','--topic','cvd.events.v1'],input=raw,text=True,stdout=subprocess.DEVNULL,check=True)
print('Submitted 10 synthetic wafers, reversed pairs and duplicate delivery',flush=True)
# Restart station while commands/results are in flight; preserve volumes.
time.sleep(5);subprocess.run(dc+['restart','metrology'],check=True,stdout=subprocess.DEVNULL)
for attempt in range(100):
 rows=[]
 for i in range(1,11):rows+=api('/metrology/runs?equipmentId=TEL-CVDTIN-08A-%02d&waferId=kla-fixture-%s-%d'%(i,tag,i))
 if len(rows)==10 and all(r['state']=='completed' for r in rows):break
 if attempt%10==0:print('Progress',len(rows),'jobs',sum(r['state']=='completed' for r in rows),'complete',flush=True)
 time.sleep(2)
else:raise AssertionError('metrology did not finish: '+str([(r['state'],r['id']) for r in rows]))
for r in rows:
 d=api('/metrology/runs/'+r['id']);m=d['result'];assert len(m['points'])==49 and isinstance(m['defects'],list);assert m['source']['waferId']==r['wafer_id'];assert m['source']['equipmentId']==r['equipment_id'];assert d['revision']==1
assert len(api('/equipment'))==10
request={'requestId':'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'};new=api('/metrology/runs/'+rows[0]['id']+'/remeasure',request);assert api('/metrology/runs/'+rows[0]['id']+'/remeasure',request)==new
print('PASS 10/10 maps; source isolation, duplicate input, station restart, fleet isolation, remeasurement dedup',flush=True)
(root/'.runtime/kla-stack-proof.json').write_text(json.dumps({'tag':tag,'rows':rows,'remeasure':new},indent=2))
