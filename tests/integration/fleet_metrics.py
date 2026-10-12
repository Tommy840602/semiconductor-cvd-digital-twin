from fleet import *
import datetime,subprocess
DC=['docker','compose','-f',os.environ.get('CVD_TEST_COMPOSE','.runtime/fleet-compose.json')]
assert DC[-1] in ('.runtime/fleet-compose.json','.runtime/secs-compose.json')
def sql(service,q):return subprocess.check_output(DC+['exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
def main():
 c=Client();samples=[]
 before={'time':time.time(),'bytes':int(sql('timescaledb',"select pg_database_size('cvd')")),'rows':int(sql('timescaledb','select count(*) from telemetry'))}
 for _ in range(12):
  f=c.call('/equipment');now=time.time();samples.append({'time':now,'maxAgeSeconds':max(now-datetime.datetime.fromisoformat(m['sourceTime'].replace('Z','+00:00')).timestamp() for m in f),'processing':sum(m['processing'] for m in f)})
  time.sleep(5)
 after={'time':time.time(),'bytes':int(sql('timescaledb',"select pg_database_size('cvd')")),'rows':int(sql('timescaledb','select count(*) from telemetry'))}
 size=sql('timescaledb',"select coalesce(sum(pg_column_size(body)),0) from telemetry where time>now()-interval '1 minute'")
 stats=subprocess.check_output(['docker','stats','--no-stream','--format','{{.Name}} {{.CPUPerc}} {{.MemUsage}}'],text=True)
 lag=subprocess.check_output(DC+['exec','-T','kafka','/opt/kafka/bin/kafka-consumer-groups.sh','--bootstrap-server','kafka:9092','--all-groups','--describe'],text=True,stderr=subprocess.DEVNULL)
 result={'before':before,'after':after,'samples':samples,'lastMinutePayloadBytes':int(size),'statistics':stats,'kafka':lag}
 pathlib.Path('.runtime/secs-metrics.json' if ':21000/' in BASE else '.runtime/fleet-metrics.json').write_text(json.dumps(result,indent=2));print('Saved load and capacity evidence')
if __name__=='__main__':main()
