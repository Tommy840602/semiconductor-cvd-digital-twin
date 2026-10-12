"""Maintenance acceptance on cvd-disposition-test only. Never target production."""
import exception_disposition as h
from exception_disposition import Client,command,wait,ROOT,IDS,PASSWORD
import json,uuid,time,subprocess,requests
h.COMPOSE=ROOT/'.runtime/maintenance-compose.json';assert json.loads(h.COMPOSE.read_text())['name']=='cvd-disposition-test'
h.DC=['docker','compose','-f',str(h.COMPOSE)]
id=IDS[7]
def run():
 wait(lambda:requests.get(h.BASE+'/auth/csrf').status_code==200,180)
 leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip());ee=Client('disposition.ee',PASSWORD);pe=Client('disposition.pe',PASSWORD);op=Client('disposition.op',PASSWORD)
 state=lambda:leader.tool(id)
 wait(lambda:state().get('maintenanceModelVersion')==1)
 print('Initial',[(s['equipmentId'],len(s.get('lots',[])),s.get('maintenance',{}).get('phase')) for s in [leader.tool(i) for i in IDS]],flush=True)
 assert not state().get('maintenance'), 'Use a fresh test machine'
 other={i:leader.tool(i)['lots'] for i in IDS if i!=id}
 assert requests.get(h.BASE+'/equipment/'+id+'/maintenance').status_code==401
 for kind in ['maintenance-request','maintenance-end','maintenance-carrier-load','qualification-start','maintenance-carrier-restore','resume']:
  command(op,id,kind,code=403)
 if not state()['lots']:
  command(leader,id,'resume')
  recipe=leader.tool(id,'/recipes');r=leader.tool(id,'/recipes',dict(expectedVersion=recipe['currentVersion'],settings={'stepSeconds':dict(preheat=1,stabilize=1,deposition=6,purge=1)}));wait(lambda:leader.tool(id,'/commands/'+r['commandId'])['status']=='succeeded')
  for n in range(1,4):command(leader,id,'start',lot=id+'-MAINT-'+uuid.uuid4().hex[:8],params={'lp':f'LP{n}','ll':f'LL{n}'})
 wait(lambda:any(p['waferId'] for p in state()['chambers']))
 def maint(kind,extra=None,client=leader):
  m=state().get('maintenance');params={'maintenanceId':m['id'],'expectedGeneration':m['generation']} if m else {};params.update(extra or {})
  for attempt in range(20):
   try:return command(client,id,kind,params=params)
   except AssertionError as ex:
    if '資料過期' not in str(ex):raise
    time.sleep(1)
  raise AssertionError('source remains stale')
 result=maint('maintenance-request',client=ee);cid=result['id'];body={'commandId':cid,'type':'maintenance-request','parameters':{}}
 # A replay after state change returns its original result; no second cycle.
 replay=leader.tool(id,'/commands',body);assert replay['status']=='succeeded'
 wait(lambda:state()['maintenance']['phase']=='maintenance',180)
 baseline=[l for l in state()['lots'] if l.get('purpose')!='dummy'];assert len(baseline)==3
 (ROOT/'.runtime/maintenance-baseline.json').write_text(json.dumps(baseline,indent=2))
 beforefleet=next(x for x in leader.call('/equipment') if x['equipmentId']==id)
 maint('maintenance-carrier-load',{'lp':'LP2'});s=state();assert s['maintenance']['loaded'] and next(l for l in s['lots'] if l['id']==s['maintenance']['loadedLotId'])['purpose']=='dummy'
 assert len(next(l for l in s['lots'] if l['id']==s['maintenance']['loadedLotId'])['wafers'])==4
 maint('maintenance-end');assert state()['dispatchPaused']
 m=state()['maintenance'];command(leader,id,'qualification-start',params={'maintenanceId':m['id'],'expectedGeneration':m['generation']-1},code=409)
 maint('qualification-start');runid=state()['maintenance']['run']['id']
 recipe=leader.tool(id,'/recipes');leader.tool(id,'/recipes',dict(expectedVersion=recipe['currentVersion'],settings={'stepSeconds':dict(preheat=2,stabilize=1,deposition=6,purge=1)}),409)
 wait(lambda:any(p['waferId'] for p in state()['chambers']))
 # Transport outage while device advances, followed by a device restart. No automatic clock restart.
 subprocess.run(h.DC+['stop','bridge'],check=True,stdout=subprocess.DEVNULL);time.sleep(3)
 subprocess.run(h.DC+['restart','simulator-08'],check=True,stdout=subprocess.DEVNULL)
 subprocess.run(h.DC+['start','bridge'],check=True,stdout=subprocess.DEVNULL)
 wait(lambda:state()['communicationQuality']=='good' and state()['paused'],90)
 assert state()['maintenance']['run']['id']==runid and state()['dispatchPaused']
 maint('recovery-clock')
 wait(lambda:state()['maintenance']['phase']=='passed',240)
 assert len(state()['maintenance']['run']['results'])==4
 maint('maintenance-carrier-restore');assert state()['maintenance']['phase']=='ready'
 after=[l for l in state()['lots'] if l.get('purpose')!='dummy']
 assert baseline==after,'Production FOUP content/progress changed'
 afterfleet=next(x for x in leader.call('/equipment') if x['equipmentId']==id)
 assert all(beforefleet[k]==afterfleet[k] for k in ['success','scrap','activeLots'])
 hist=wait(lambda:(lambda v:v if v['history'] and v['history'][0]['phase']=='ready' else None)(leader.tool(id,'/maintenance')))
 events=leader.tool(id,'/maintenance/'+state()['maintenance']['id']+'/events');assert any(x.get('actor') for x in events)
 maint('resume');assert state()['maintenance']['phase']=='released'
 print('PASS 3 occupied LPs, fixed PMs, exact FOUP restore, replay, restart/outage, API permissions, unchanged production counters',flush=True)
 # A second cycle: faulted Dummy must use the existing EE/PE/Leader chain.
 maint('maintenance-request');wait(lambda:state()['maintenance']['phase']=='maintenance',180);maint('maintenance-end');maint('maintenance-carrier-load',{'lp':'LP1'});maint('qualification-start')
 p=wait(lambda:next((p for p in state()['chambers'] if p['waferId'] and p.get('processTelemetry',{}).get('elapsedSeconds',0)>0),None));command(leader,id,'fault',p['id'])
 wafer=p['waferId'];auto=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['wafer_id']==wafer),None))
 d=ee.tool(id,'/recoveries',dict(incidentId=auto['incident_id'],pmId=p['id'],dispositionType='scrap',cause='Dummy 模擬故障',correctiveAction='已排除故障',verificationMethod='檢查模擬數據',verificationResult='數據恢復',stepSeconds=[],targetThicknessNm=10));rid=d['id'];rev=d['revision']
 ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':rev});pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':rev});command(leader,id,'clear',p['id']);command(leader,id,'recovery-clock')
 wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='awaiting_result');inc=state()['incidents'][auto['incident_id']]
 pe.tool(id,'/dispositions/'+rid+'/confirm',dict(revision=rev,executionId=inc['executionId'],resultVersion=inc['resultVersion']))
 wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='resolved')
 view=wait(lambda:(lambda v:v if v['eligible'] else None)(leader.tool(id,'/release')));leader.tool(id,'/release',{'generation':view['generation']});wait(lambda:not state()['dispositionHold'])
 assert state()['dispatchPaused'] and state()['maintenance']['phase']!='released'
 wait(lambda:state()['maintenance']['phase']=='failed',240)
 failrun=state()['maintenance']['run'];assert any(not r['complete'] for r in failrun['results'].values())
 maint('qualification-start');wait(lambda:state()['maintenance']['phase']=='passed',240)
 assert state()['maintenance']['run']['id']!=failrun['id'] and len(state()['maintenance']['run']['results'])==4
 maint('maintenance-carrier-restore')
 assert all(leader.tool(i)['lots']==other[i] for i in other)
 final=state();evidence={'initialBaseline':baseline,'firstHistory':hist,'events':events,'faultRecovery':ee.tool(id,'/recoveries/'+rid),'final':final,'isolatedTools':list(other)}
 (ROOT/'.runtime/maintenance-evidence.json').write_text(json.dumps(evidence,indent=2,default=str))
 print('PASS Dummy fault → EE proposal → PE approval/confirmation → Leader release retains maintenance lock → all-four retest; other nine tools unchanged',flush=True)
 # Keep final ready state for browser verification; do not auto-resume.
if __name__=='__main__':run()
