import hashlib,json,logging,os,subprocess,sys,tempfile,time
from pathlib import Path
from urllib.request import Request,urlopen
from urllib.error import HTTPError
from kafka import KafkaConsumer,KafkaProducer
from engine import Engine
logging.basicConfig(level=logging.INFO,format='%(asctime)s %(message)s')
logging.getLogger('kafka').setLevel(logging.WARNING)
base=os.environ.get('VISION_BACKEND','http://backend:8080');token=Path(os.environ.get('VISION_TOKEN_FILE','/run/secrets/vision_token')).read_text().strip()
images=os.environ.get('VISION_IMAGES','/images');brokers=os.environ.get('KAFKA_SERVERS','kafka:9092').split(',')
def request(path,data=None,lease=None):
 headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'}
 if lease:headers['X-Vision-Lease']=lease
 with urlopen(Request(base+'/api/vision/worker/'+path,data=data,headers=headers),timeout=5) as r:return r.read(2_000_001)
def run():
 consumer=producer=engine=None
 try:
  engine=Engine(images)
  consumer=KafkaConsumer('vision.jobs.v1',bootstrap_servers=brokers,group_id='vision-worker-v1',auto_offset_reset='earliest',enable_auto_commit=False,max_poll_records=1,max_poll_interval_ms=120000)
  producer=KafkaProducer(bootstrap_servers=brokers,acks='all',retries=5,max_block_ms=10000)
  while True:
   batches=consumer.poll(timeout_ms=1000);Path('/tmp/vision-heartbeat').touch()
   for messages in batches.values():
    for message in messages:
     event=json.loads(message.value);id=event.get('jobId','')
     try:claim=json.loads(request('jobs/'+id+'/claim',b'{}'))
     except HTTPError as e:
      if e.code==409:consumer.commit();continue
      raise
     result={'jobId':id,'lease':claim['lease'],'state':'failed'}
     try:
      raw=request('jobs/'+id+'/input',lease=claim['lease']);in_=claim['input']
      if len(raw)>2_000_000 or hashlib.sha256(raw).hexdigest()!=in_['hash']:raise ValueError('input-hash')
      output=engine.analyze(raw,claim['kind'],in_.get('calibration'));output['inputHash']=in_['hash'];result.update(state='completed',result=output)
     except Exception as e:
      logging.warning('Analysis failed for %s (%s)',id,type(e).__name__)
      if not engine.process.is_alive():engine=Engine(images)
     producer.send('vision.results.v1',key=id.encode(),value=json.dumps(result,allow_nan=False).encode()).get(timeout=5)
     consumer.commit();Path('/tmp/vision-heartbeat').touch()
 finally:
  if engine:engine.close()
  if consumer:consumer.close()
  if producer:producer.close(timeout=5)
if __name__=='__main__':
 while True:
  try:run()
  except Exception as e:logging.warning('Worker reconnecting (%s)',type(e).__name__);time.sleep(3)
