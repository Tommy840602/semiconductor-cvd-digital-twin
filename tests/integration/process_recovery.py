"""Isolated live recovery while normal batches remain in progress."""
from scenarios import *
import subprocess,datetime
DC=['docker','compose','-f','.runtime/process-compose.json']
def dc(*a):return subprocess.check_output(DC+list(a),text=True)
def sql(service,q):return dc('exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc',q).strip()
def fresh():
 try:
  s=request('/equipment');return s if time.time()-datetime.datetime.fromisoformat(s['sourceTime'].replace('Z','+00:00')).timestamp()<5 else None
 except Exception:return None
def stored_after(before):
 try:return int(sql('timescaledb','select count(*) from telemetry'))>before
 except subprocess.CalledProcessError:return False
for service in os.getenv('RECOVERY_SERVICES','gateway,mqtt,kafka,postgres,timescaledb').split(','):
 before=int(sql('timescaledb','select count(*) from telemetry'))
 dc('stop',service)
 try:time.sleep(7)
 finally:dc('start',service)
 wait(fresh,90)
 wait(lambda:stored_after(before),90)
 print('PASS recover '+service,flush=True)
# Capture exact durable state after stopping; upgrade to the tested candidate.
dc('stop','simulator')
dc('cp','simulator:/data/state.json','.runtime/recovery-checkpoint.json')
saved=json.load(open('.runtime/recovery-checkpoint.json'))
dc('up','-d','--no-deps','--no-build','simulator')
s=wait(lambda:(s if (s:=request('/equipment')).get('paused') else None),90)
for p in s['chambers']:
 before=next(x for x in saved['chambers'] if x['id']==p['id'])
 assert p.get('processTelemetry')==before.get('processTelemetry'),(p,before)
assert s['tick']==saved['tick'] if 'tick' in s else True
before=[p.get('processTelemetry') for p in s['chambers']];time.sleep(3)
assert before==[p.get('processTelemetry') for p in request('/equipment')['chambers']]
command('resume')
wait(lambda:not request('/equipment')['paused'],30)
print('PASS checkpoint: exact phase/thickness preserved; stays paused until manual resume',flush=True)
assert sql('timescaledb','select count(*)-count(distinct id) from telemetry')=='0'
assert sql('postgres','select count(*)-count(distinct id) from history')=='0'
print('PASS no duplicated events after outages',flush=True)
