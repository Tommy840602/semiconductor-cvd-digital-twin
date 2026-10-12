"""Cancellation transaction race plus completed-command redelivery, isolated SECS only."""
from fleet import *
import subprocess
assert ':21000/' in BASE
DC=['docker','compose','-f','.runtime/secs-compose.json']
def main():
 c=Client();ee=Client('fleet.ee','Fleet-Recovery-Test-2026!');pe=Client('fleet.pe','Fleet-Recovery-Test-2026!');id=IDS[0]
 previous=json.loads(pathlib.Path('.runtime/secs-recovery.json').read_text())
 pending=next((r for r in ee.tool(id,'/recoveries') if r['status']=='approved_waiting'),None)
 if pending:
  detail=ee.tool(id,'/recoveries/'+pending['id']);assert not detail['intents'][0]['published']
  ee.tool(id,'/recoveries/'+pending['id']+'/withdraw',{'revision':pending['revision']})
  assert ee.tool(id,'/recoveries/'+pending['id'])['status']=='withdrawn'
 p=next(p for p in c.tool(id)['chambers'] if p['fault'] and p.get('waferId'));t=p['processTelemetry'];baseline=t['thicknessNm']
 d=ee.tool(id,'/recoveries',dict(pmId=p['id'],stepSeconds=[0,0,2,1],targetThicknessNm=baseline+t['rootDepositionRateNmPerSecond']*2,troubleshooting='核准與撤回交易競態測試'));rid=d['id'];ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':1})
 def race(client,action):
  r=client.s.post(BASE+'/equipment/'+id+'/recoveries/'+rid+'/'+action,json={'revision':1},headers={'X-CSRF-Token':client.csrf},timeout=15);return r.status_code
 with concurrent.futures.ThreadPoolExecutor(2) as pool:
  a=pool.submit(race,pe,'approve');w=pool.submit(race,ee,'withdraw');codes=[a.result(),w.result()]
 assert codes[0] in (200,409) and codes[1]==200,codes
 d=ee.tool(id,'/recoveries/'+rid);assert d['status']=='withdrawn';assert all(not i['published'] for i in d['intents'])
 # Replay the already completed authorized execution without changing its identity or prescription.
 eid=previous['execution'];uuid.UUID(eid);cid=eid+'-execute'
 subprocess.run(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-c',f"UPDATE commands SET status='received',last_sent=NULL WHERE id='{cid}'"],check=True,stdout=subprocess.DEVNULL)
 wait(lambda:c.tool(id,'/commands/'+cid)['status']=='succeeded')
 s=c.tool(id);assert s['recoveries'][eid]['status']=='completed';p=next(x for x in s['chambers'] if x['id']==p['id']);assert p['processTelemetry']['thicknessNm']==baseline
 pathlib.Path('.runtime/secs-cancel.json').write_text(json.dumps({'request':rid,'raceCodes':codes,'withdrawn':True,'completedExecutionReplayUnchanged':True,'baselinePreserved':True},indent=2));print('PASS: concurrent approval/withdrawal and completed-command replay')
if __name__=='__main__':main()
