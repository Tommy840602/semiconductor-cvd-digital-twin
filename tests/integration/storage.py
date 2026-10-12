"""Additional storage idempotency and temporal-query checks."""
from scenarios import *
import subprocess, datetime

def sql(service,query):return subprocess.check_output(['docker','compose','exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc',query],text=True).strip()
def main():
    history=sql('postgres',"SELECT body::text FROM history WHERE body ? 'eventId' ORDER BY time LIMIT 1")
    e=json.loads(history)
    count=sql('postgres',"SELECT count(*) FROM history WHERE id='"+e['eventId']+"'")
    subprocess.run(['docker','compose','exec','-T','mqtt','mosquitto_pub','-t','cvd/CVD-01/events','-m',history,'-q','1'],check=True)
    time.sleep(3)
    assert sql('postgres',"SELECT count(*) FROM history WHERE id='"+e['eventId']+"'")==count
    from urllib.parse import urlencode
    end=datetime.datetime.now(datetime.timezone.utc);start=end-datetime.timedelta(hours=2)
    result=request('/telemetry?'+urlencode({'from':start.isoformat(),'to':end.isoformat(),'pmId':'PM2'}))
    assert result and all(all(p['id']=='PM2' for p in e['payload']['chambers']) for e in result)
    result=request('/history?page=0');assert len(result)<=100
    if len(result)==100:
        nextpage=request('/history?page=1');assert not {e['eventId'] for e in result}&{e['eventId'] for e in nextpage}
    print('PASS duplicate delivery, PM-specific aggregation, and history pagination',flush=True)
if __name__=='__main__':main()
