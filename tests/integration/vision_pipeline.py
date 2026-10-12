"""Isolated Compose fixture: actual KLA-result consumer -> DB -> Kafka -> OpenCV -> DB."""
import json,subprocess,time,uuid,urllib.request,http.cookiejar,os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
COMPOSE=['docker','compose','-p','cvd-vision-proof','-f',str(ROOT/'.runtime/vision-proof.compose.yaml')]
def sql(query):return subprocess.check_output(COMPOSE+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-At','-c',query],text=True).strip()
def send(events):
 code="from kafka import KafkaProducer;import sys,json;p=KafkaProducer(bootstrap_servers='kafka:9092',acks='all');[p.send('metrology.events.v1',value=json.dumps(e).encode()).get(timeout=15) for e in json.load(sys.stdin)];p.close()"
 result=subprocess.run(COMPOSE+['exec','-T','vision','python','-c',code],input=json.dumps(events),text=True,capture_output=True)
 if result.returncode:raise RuntimeError(result.stderr[-3000:])
def fixture():
 before=int(sql("SELECT count(*) FROM vision_runs"));completedBefore=int(sql("SELECT count(*) FROM vision_jobs WHERE state='completed'")); events=[];ids=[]
 for n in range(1,11):
  mid=str(uuid.uuid4());ids.append(mid);eq=f'TEL-CVDTIN-08A-{n:02d}'
  source=dict(equipmentId=eq,waferId='vision-wafer-'+str(n),lotId='vision-lot',slot=n,pm='PM1',purpose='product',processRunId='vision-run-'+str(n),recipeVersion='fixture-v1',thicknessNm=10,sampleThicknessNm=[10]*5,targetThicknessNm=10,thicknessTolerancePct=10,uniformityLimitPct=10)
  sql("INSERT INTO metrology_runs(id,source_key,revision,equipment_id,wafer_id,source,state) VALUES('%s','%s',1,'%s','%s','%s'::jsonb,'measuring')"%(mid,mid,eq,source['waferId'],json.dumps(source)))
  result=dict(modelVersion='synthetic-kla-v1',seed='a'*64,synthetic=True,source=source,diameterMm=300,edgeExclusionMm=3,coordinates='center-x-right-y-up-notch-down-mm',points=[dict(x=x*25,y=y*25,thicknessNm=10) for x in range(-3,4) for y in range(-3,4)],defects=[],meanNm=10,minNm=10,maxNm=10,uniformityPct=0,thicknessQuality='pass',defectQuality='pass',quality='pass',defectCountLimit=10,defectSizeLimitUm=1)
  events.append(dict(equipmentId='KLA-SIM-01',eventId='vision-'+mid,kind='measurement-state',sequence=n,payload=dict(measurementId=mid,state='completed',result=result)))
 start=time.monotonic();send(events+events)
 for _ in range(120):
  if sql("SELECT count(*) FROM vision_jobs WHERE state='completed'")==str(completedBefore+20):break
  time.sleep(1)
 else:raise RuntimeError(sql('SELECT state,count(*) FROM vision_jobs GROUP BY state'))
 assert sql('SELECT count(*) FROM vision_runs')==str(before+10);assert sql('SELECT count(*) FROM commands')=='0';assert int(sql("SELECT count(*) FROM metrology_runs WHERE state='completed'"))>=10
 duration=time.monotonic()-start
 proof={'equipmentCount':10,'jobs':20,'complete':20,'seconds':round(duration,2),'commandsCreated':0,'inputModes':sql("SELECT input->>'mode',count(*) FROM vision_jobs GROUP BY 1"),'ids':ids}
 (ROOT/'.runtime/vision-pipeline-proof.json').write_text(json.dumps(proof,indent=2));print(json.dumps(proof))
if __name__=='__main__':fixture()
