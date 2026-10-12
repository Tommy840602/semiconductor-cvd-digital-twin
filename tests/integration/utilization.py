"""Read metrics and exercise isolated tool 10 through the complete SECS pipeline."""
import json,pathlib,subprocess,time,sys
sys.path.insert(0,str(pathlib.Path(__file__).parent))
import exception_disposition as base
base.COMPOSE=base.ROOT/'.runtime/utilization-compose.json'
base.DC=['docker','compose','-f',str(base.COMPOSE)]
assert json.loads(base.COMPOSE.read_text())['name']=='cvd-disposition-test'
def ts(q):
 return subprocess.check_output(base.DC+['exec','-T','timescaledb','psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
def run():
 base.wait(lambda:base.requests.get(base.BASE+'/auth/csrf').status_code==200)
 assert base.requests.get(base.BASE+'/equipment/utilization').status_code==401
 c=base.Client('cvd.leader',(base.ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip())
 op=base.Client('disposition.op',base.PASSWORD)
 counts=base.sql('select count(*) from commands')
 ee=base.Client('disposition.ee',base.PASSWORD)
 for who in [c,op,ee]:
  for period in ['today','24h','7d','30d']:
   r=who.call('/equipment/utilization?period='+period);assert len(r['equipment'])==10;assert r['fleet']['quality']=='partial';assert r['fleet']['utilization'] is None
 c.call('/equipment/utilization?period=invalid',code=400)
 assert base.sql('select count(*) from commands')==counts
 base.wait(lambda:int(ts('select count(distinct equipment_id) from utilization_second'))==10)
 id=base.IDS[-2]
 base.command(c,id,'resume')
 state=c.tool(id)
 if not any(l['phase']!='complete' for l in state['lots']):base.command(c,id,'start',lot=id+'-util-'+str(int(time.time())),params={'lp':'LP1','ll':'LL1'})
 base.wait(lambda:any(p['remaining']>0 and not p['fault'] for p in c.tool(id)['chambers']))
 base.wait(lambda:int(ts("select count(*) from utilization_second where equipment_id='"+id+"' and mask>0"))>0)
 subprocess.run(base.DC+['restart','simulator-09'],check=True,stdout=subprocess.DEVNULL)
 base.wait(lambda:c.tool(id).get('paused') is True);time.sleep(3)
 assert int(ts("select mask from utilization_second where equipment_id='"+id+"' order by time desc limit 1"))==0
 # The same state remaining frozen must not count as busy; then dispatch hold allows existing processing.
 base.command(c,id,'resume')
 for lot in c.tool(id)['lots']:
  if lot['phase']!='complete':base.command(c,id,'pause',lot=lot['id'])
 base.wait(lambda:int(ts("select mask from utilization_second where equipment_id='"+id+"' order by time desc limit 1"))>0)
 active=next(p for p in c.tool(id)['chambers'] if p['remaining']>0 and not p['fault'])
 base.command(c,id,'fault',active['id']);time.sleep(3)
 bit=1<<(int(active['id'][2:])-1)
 assert int(ts("select mask from utilization_second where equipment_id='"+id+"' order by time desc limit 1"))&bit==0
 # Stop only the isolated bridge; durable source events must backfill after restart.
 before=ts("select max(time) from utilization_second where equipment_id='"+id+"'")
 subprocess.run(base.DC+['stop','bridge'],check=True,stdout=subprocess.DEVNULL)
 time.sleep(5)
 subprocess.run(base.DC+['start','bridge'],check=True,stdout=subprocess.DEVNULL)
 base.wait(lambda:int(ts("select count(*) from utilization_second where equipment_id='"+id+"' and time>'"+before+"'::timestamptz and time<='"+before+"'::timestamptz+interval '5 seconds'"))==5)
 duplicates=ts('select count(*) from (select equipment_id,time,count(*) from utilization_second group by equipment_id,time having count(*)>1) x');assert duplicates=='0'
 subprocess.run(base.DC+['restart','simulator-09'],check=True,stdout=subprocess.DEVNULL)
 evidence={'allTools':10,'roles':['Leader','Operator','Engineer'],'periods':4,'readOnly':True,'pausedMask':0,'faultExcluded':True,'bridgeBackfill':True,'duplicateSeconds':0}
 (base.ROOT/'.runtime/utilization-integration-proof.json').write_text(json.dumps(evidence,indent=2));print(json.dumps(evidence))
if __name__=='__main__':run()
