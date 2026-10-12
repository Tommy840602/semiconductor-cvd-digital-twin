"""Run only against this Compose demo. Restores every service it stops."""
from scenarios import *
import subprocess, datetime

def dc(*args):return subprocess.check_output(['docker','compose',*args],text=True)
def sql(service,query):return dc('exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc',query).strip()
def healthy_snapshot():
    try:
        s=request('/equipment')
        stamp=datetime.datetime.fromisoformat(s['sourceTime'].replace('Z','+00:00')).timestamp()
        return s if time.time()-stamp<5 else None
    except Exception:return None

def outage(service):
    before=int(sql('timescaledb','select count(*) from telemetry'))
    dc('stop',service)
    try:time.sleep(7)
    finally:dc('start',service)
    wait(healthy_snapshot,90)
    wait(lambda:int(sql('timescaledb','select count(*) from telemetry'))>before,90)
    print('PASS service recovery: '+service,flush=True)

def main():
    wait(healthy_snapshot,60)
    command('resume')
    for service in ['mqtt','kafka','postgres','timescaledb']:outage(service)
    dc('exec','-T','redis','redis-cli','DEL','cvd:latest')
    request('/equipment');assert dc('exec','-T','redis','redis-cli','EXISTS','cvd:latest').strip()=='1'
    print('PASS Redis rebuild',flush=True)
    # Replayed identical telemetry and commands do not duplicate stored identities.
    assert sql('timescaledb','select count(*)-count(distinct id) from telemetry')=='0'
    assert sql('postgres','select count(*)-count(distinct id) from history')=='0'
    name='RESTART-'+uuid.uuid4().hex[:6]
    command('start',lot=name,parameters={'lp':'LP1','ll':'LL1'})
    wait(lambda:request('/equipment')['atm']['waferId'],30)
    dc('restart','simulator')
    s=wait(lambda: (s if (s:=request('/equipment')).get('paused') else None),40)
    l=next(l for l in s['lots'] if l['id']==name);assert len(l['wafers'])==len({w['id'] for w in l['wafers']})==25
    positions={w['id']:w['location'] for w in l['wafers']};time.sleep(3)
    assert positions=={w['id']:w['location'] for l in request('/equipment')['lots'] if l['id']==name for w in l['wafers']}
    # Old terminal history is removed; active-lot history remains for state integrity.
    sql('postgres',"INSERT INTO history(id,time,lot_id,kind,body) VALUES ('retention-old',now()-interval '40 days','','fixture','{}'),('retention-active',now()-interval '40 days','"+name+"','fixture','{}') ON CONFLICT DO NOTHING")
    sql('timescaledb',"INSERT INTO telemetry(time,id,body) VALUES(now()-interval '40 days','retention-old','{\"payload\":{\"chambers\":[]}}') ON CONFLICT DO NOTHING")
    dc('restart','backend');wait(healthy_snapshot,60)
    wait(lambda:sql('postgres',"select count(*) from history where id='retention-old'")=='0',30)
    assert sql('postgres',"select count(*) from history where id='retention-active'")=='1'
    future=datetime.datetime.now(datetime.timezone.utc).isoformat();old=(datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=45)).isoformat()
    from urllib.parse import urlencode
    rows=request('/telemetry?'+urlencode({'from':old,'to':future}));assert all(r.get('eventId')!='retention-old' for r in rows)
    assert request('/telemetry?'+urlencode({'from':old,'to':(datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=35)).isoformat()}))==[]
    job=sql('timescaledb',"SELECT job_id FROM timescaledb_information.jobs WHERE proc_name='policy_retention' AND hypertable_name='telemetry' LIMIT 1")
    sql('timescaledb',"CALL run_job("+str(int(job))+")")
    assert sql('timescaledb',"select count(*) from telemetry where id='retention-old'")=='0'
    print('PASS retention and active-state protection',flush=True)
    command('resume',lot=name)
    wait(lambda:next(l for l in request('/equipment')['lots'] if l['id']==name)['phase']=='complete',180)
    print('PASS simulator checkpoint and manual resume',flush=True)
    # Validate transactional acceptance rollback on the actual PostgreSQL schema.
    sql('postgres',"BEGIN; INSERT INTO commands(id,body,status) VALUES('rollback-fixture','{}','received'); ROLLBACK;")
    assert sql('postgres',"select count(*) from commands where id='rollback-fixture'")=='0'
    print('PASS PostgreSQL acceptance rollback',flush=True)
if __name__=='__main__':main()
