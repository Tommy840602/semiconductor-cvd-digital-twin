"""Fault injection for approval transactions, transport retry, cancellation and retention.
Run only against the isolated recovery compose after wafer_recovery.py.
"""
import os,sys
os.environ.setdefault('CVD_COMPOSE','.runtime/recovery-compose.json')
from wafer_recovery import *
def main():
 leader=Client();leader.login('cvd.leader',PWD);ee=Client();ee.login('recovery.ee',UI_PASSWORD);pe=Client();pe.login('recovery.pe',UI_PASSWORD)
 # Finish any submitted browser fixture by withdrawing it; failed/success records stay immutable.
 for r in ee.call('/recoveries'):
  if r['status'] in ['draft','submitted','returned','approved_waiting'] and r['owner_id']==ee.call('/auth/me')['id']:ee.call('/recoveries/'+r['id']+'/withdraw',{'revision':r['revision']})
 command(leader,'fault','PM1')
 def new(steps=[0,0,0,1]):
  d=ee.call('/recoveries',{'pmId':'PM1','stepSeconds':steps,'targetThicknessNm':100,'troubleshooting':'隔離環境可靠性測試，保留既有五點膜厚'});ee.call('/recoveries/'+d['id']+'/submit',{'revision':d['revision']});return d['id'],d['revision']
 rid,rev=new()
 sql("CREATE OR REPLACE FUNCTION reject_test_intent() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'intent rollback test'; END $$; CREATE TRIGGER recovery_test_rollback BEFORE INSERT ON recovery_intents FOR EACH ROW EXECUTE FUNCTION reject_test_intent()")
 try:pe.call('/recoveries/'+rid+'/approve',{'revision':rev},code=503)
 finally:sql('DROP TRIGGER recovery_test_rollback ON recovery_intents; DROP FUNCTION reject_test_intent()')
 d=pe.call('/recoveries/'+rid);assert d['status']=='submitted' and not d['intents'];assert not any(e['action']=='approved' for e in d['events'])
 pe.call('/recoveries/'+rid+'/approve',{'revision':rev})
 subprocess.run(DC+['stop','gateway'],check=True,stdout=subprocess.DEVNULL)
 try:
  cid=str(uuid.uuid4());leader.call('/commands',{'commandId':cid,'type':'clear','targetId':'PM1'})
  time.sleep(7);d=leader.call('/recoveries/'+rid);assert d['status']=='approved_waiting' and '通訊' in d['reason'],d
 finally:subprocess.run(DC+['start','gateway'],check=True,stdout=subprocess.DEVNULL)
 if leader.call('/equipment')['paused']:command(leader,'recovery-clock')
 d=wait(lambda:(lambda d:d if d['status']=='failed' else None)(leader.call('/recoveries/'+rid)))
 assert any(e['action']=='executing' for e in d['events']), '1-second runs must retain a start event'
 execution=d['intents'][0]['execution_id'];before=leader.call('/equipment')['chambers'][0]['processTelemetry']
 sql("UPDATE commands SET status='received',result=NULL,last_sent=NULL WHERE id='"+execution+"-execute'")
 wait(lambda:leader.call('/commands/'+execution+'-execute')['status']=='succeeded')
 after=leader.call('/equipment')['chambers'][0]['processTelemetry'];assert before['thicknessNm']==after['thicknessNm'] and before['processRunId']==after['processRunId']
 print('PASS approval rollback, communication auto-recovery, 1-second start audit and ACK-loss replay',flush=True)
 # Freeze transport after a fresh snapshot. Publish once, restart device into
 # paused state, then cancel. Device confirmation must fence the queued command.
 rid,rev=new([0,0,10,0]);before=leader.call('/equipment')['chambers'][0]['processTelemetry']['thicknessNm']
 subprocess.run(DC+['pause','gateway'],check=True,stdout=subprocess.DEVNULL)
 try:
  pe.call('/recoveries/'+rid+'/approve',{'revision':rev})
  wait(lambda:leader.call('/recoveries/'+rid)['intents'][0]['published'],timeout=4)
  subprocess.run(DC+['restart','simulator'],check=True,stdout=subprocess.DEVNULL)
  d=ee.call('/recoveries/'+rid+'/withdraw',{'revision':rev});assert d['status']=='cancelling'
  ee.call('/recoveries/'+rid+'/revise',{'revision':rev},code=409)
 finally:subprocess.run(DC+['unpause','gateway'],check=True,stdout=subprocess.DEVNULL)
 d=wait(lambda:(lambda d:d if d['status']=='withdrawn' else None)(leader.call('/recoveries/'+rid)))
 assert d['intents'][0]['result']['status']=='cancelled'
 assert leader.call('/equipment')['chambers'][0]['processTelemetry']['thicknessNm']==before
 command(leader,'recovery-clock');time.sleep(2);assert leader.call('/equipment')['chambers'][0]['processTelemetry']['thicknessNm']==before
 print('PASS published cancellation requires device confirmation and fences late execution',flush=True)
 rid,rev=new([0,0,15,0]);pe.call('/recoveries/'+rid+'/approve',{'revision':rev});wait(lambda:leader.call('/recoveries/'+rid)['status']=='executing')
 ee.call('/recoveries/'+rid+'/withdraw',{'revision':rev},code=409);ee.call('/recoveries/'+rid+'/save',{'revision':rev},code=409)
 subprocess.run(DC+['restart','backend'],check=True,stdout=subprocess.DEVNULL)
 wait(lambda:__import__('requests').get('http://localhost:19080/actuator/health').status_code==200)
 # Backend restart cannot create another execution ID; complete or fail naturally.
 d=wait(lambda:(lambda d:d if d['status']=='failed' else None)(leader.call('/recoveries/'+rid)));assert len(d['intents'])==1
 print('PASS started execution immutable and backend restart retains one intent',flush=True)
 # Race approval vs withdrawal while a fault prevents start.
 command(leader,'fault','PM1');rid,rev=new()
 def race(c,action):
  r=c.s.post('http://localhost:19000/api/recoveries/'+rid+'/'+action,json={'revision':rev},headers={'X-CSRF-Token':c.csrf});return r.status_code
 with concurrent.futures.ThreadPoolExecutor(2) as pool:results=list(pool.map(lambda x:race(*x),[(pe,'approve'),(ee,'withdraw')]))
 assert results[0] in [200,409] and results[1]==200;assert leader.call('/recoveries/'+rid)['status']=='withdrawn'
 print('PASS concurrent approval/withdrawal has one terminal outcome',flush=True)
 # Protect an active wafer's old event; expire an unrelated complete chain as a unit.
 rid,rev=new();active=leader.call('/recoveries/'+rid)['wafer_id'];fixture='retention-'+uuid.uuid4().hex
 sql("INSERT INTO history(id,time,wafer_id,kind,body) VALUES('"+fixture+"-active',now()-interval '31 days','"+active+"','recovery-test','{}'); INSERT INTO recovery_requests(id,wafer_id,owner_id,revision,status,created_at,updated_at) VALUES('"+fixture+"','old-wafer','test',1,'completed',now()-interval '31 days',now()-interval '31 days'); INSERT INTO recovery_versions VALUES('"+fixture+"',1,'{}','{}',now()-interval '31 days'); INSERT INTO recovery_events(request_id,revision,action,actor,details,time) VALUES('"+fixture+"',1,'completed','{}','{}',now()-interval '31 days'); INSERT INTO recovery_intents(execution_id,request_id,revision,approved_by,created_at) VALUES('"+fixture+"','"+fixture+"',1,'{}',now()-interval '31 days')")
 subprocess.run(DC+['restart','backend'],check=True,stdout=subprocess.DEVNULL)
 wait(lambda:__import__('requests').get('http://localhost:19080/actuator/health').status_code==200)
 wait(lambda:sql("SELECT count(*) FROM recovery_requests WHERE id='"+fixture+"'")=='0')
 assert sql("SELECT count(*) FROM history WHERE id='"+fixture+"-active'")=='1'
 assert len(leader.call('/recoveries/'+rid)['versions'])==1
 wait(lambda:time.time()-__import__('datetime').datetime.fromisoformat(leader.call('/equipment')['sourceTime'].replace('Z','+00:00')).timestamp()<4)
 # Scrap invalidates an approved waiting proposal without starting a supplement.
 pe.call('/recoveries/'+rid+'/approve',{'revision':rev})
 subprocess.run(DC+['stop','simulator'],check=True,stdout=subprocess.DEVNULL)
 subprocess.run(DC+['start','simulator'],check=True,stdout=subprocess.DEVNULL)
 wait(lambda:leader.call('/equipment')['paused']);command(leader,'clear','PM1');command(leader,'scrap','PM1')
 wait(lambda:leader.call('/recoveries/'+rid)['status']=='invalidated')
 print('PASS active history protected, expired chain cleanup and scrap invalidation',flush=True)
if __name__=='__main__':main()
