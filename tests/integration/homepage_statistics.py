"""Statistics acceptance on the isolated SECS/Sparkplug stack, never production."""
import exception_disposition as h
from exception_disposition import Client,command,wait,ROOT,IDS,PASSWORD
import json,uuid,time,requests,subprocess
h.COMPOSE=ROOT/'.runtime/statistics-compose.json';assert json.loads(h.COMPOSE.read_text())['name']=='cvd-disposition-test'
h.DC=['docker','compose','-f',str(h.COMPOSE)]
def run():
 wait(lambda:requests.get(h.BASE+'/auth/csrf').status_code==200,180)
 leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip());ee=Client('disposition.ee',PASSWORD);pe=Client('disposition.pe',PASSWORD);op=Client('disposition.op',PASSWORD)
 assert requests.get(h.BASE+'/equipment/statistics').status_code==401
 before=h.sql('SELECT count(*) FROM commands')
 for c in [leader,ee,op]:
  r=c.call('/equipment/statistics');assert len(r['equipment'])==10 and r['end']==r['utilization']['end'];c.call('/equipment/statistics?period=invalid',code=400);c.call('/equipment/statistics?equipmentId=bad',code=400)
 assert h.sql('SELECT count(*) FROM commands')==before
 id=IDS[6];state=lambda:leader.tool(id);other={i:leader.tool(i)['lots'] for i in IDS if i!=id}
 assert not state()['lots'] or all('-STAT-' in l['id'] for l in state()['lots']), 'Use statistics fixture only'
 if not state()['lots']:
  command(leader,id,'resume');recipe=leader.tool(id,'/recipes');r=leader.tool(id,'/recipes',dict(expectedVersion=recipe['currentVersion'],settings={'stepSeconds':dict(preheat=1,stabilize=1,deposition=30,purge=1)}));wait(lambda:leader.tool(id,'/commands/'+r['commandId'])['status']=='succeeded')
  command(leader,id,'start',lot=id+'-STAT-'+uuid.uuid4().hex[:8],params={'lp':'LP1','ll':'LL1'})
 wait(lambda:int(h.sql(f"SELECT count(*) FROM statistics_closures WHERE equipment_id='{id}'"))>0,120)
 evidence={}
 wait(lambda:all(r['status']!='returning' for r in ee.tool(id,'/recoveries')),240)
 for kind in ['supplement','scrap']:
  prior=next((r for r in ee.tool(id,'/recoveries') if r['prescription'].get('dispositionType')==kind and r['status']=='resolved'),None)
  if prior:
   evidence[kind]=prior
   if state()['dispositionHold']:
    view=wait(lambda:(lambda x:x if x['eligible'] else None)(leader.tool(id,'/release')));leader.tool(id,'/release',{'generation':view['generation']});wait(lambda:not state()['dispositionHold'])
   print('PASS resumed',kind,flush=True);continue
  p=wait(lambda:next((p for p in state()['chambers'] if p['waferId'] and p['remaining']>15 and p.get('processTelemetry',{}).get('thicknessNm',0)>0),None));command(leader,id,'fault',p['id']);p=next(x for x in state()['chambers'] if x['id']==p['id']);wafer=p['waferId'];auto=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['wafer_id']==wafer),None));source=p['processTelemetry']
  body=dict(incidentId=auto['incident_id'],pmId=p['id'],dispositionType=kind,cause='統計驗證故障',correctiveAction='修復模擬氣體供應',verificationMethod='比對即時數據',verificationResult='符合模擬條件',stepSeconds=[0,0,2,1] if kind=='supplement' else [],targetThicknessNm=source['thicknessNm']+source['rootDepositionRateNmPerSecond']*2)
  d=ee.tool(id,'/recoveries',body);rid=d['id'];rev=d['revision'];ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':rev});pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':rev});command(leader,id,'clear',p['id']);command(leader,id,'recovery-clock');wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='awaiting_result')
  query=f"SELECT count(*) FROM statistics_closures WHERE equipment_id='{id}' AND wafer_id='{wafer}'"
  assert h.sql(query)=='0','Counted before PE confirmation'
  inc=state()['incidents'][auto['incident_id']];pe.tool(id,'/dispositions/'+rid+'/confirm',dict(revision=rev,executionId=inc['executionId'],resultVersion=inc['resultVersion']));wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='resolved');wait(lambda:h.sql(query)=='1')
  evidence[kind]=json.loads(h.sql(f"SELECT body FROM statistics_closures WHERE equipment_id='{id}' AND wafer_id='{wafer}'"))
  view=wait(lambda:(lambda x:x if x['eligible'] else None)(leader.tool(id,'/release')));leader.tool(id,'/release',{'generation':view['generation']});wait(lambda:not state()['dispositionHold']);assert h.sql(query)=='1'
  print('PASS',kind,'PE/return closure exactly once',flush=True)
 command(leader,id,'pause',lot=state()['lots'][0]['id'])
 # A fresh maintenance cycle has only four Dummy wafers, never formal output.
 dummy=IDS[4];ds=lambda:leader.tool(dummy);assert not ds()['lots'] and not ds().get('maintenance')
 def maint(kind,extra=None):
  m=ds().get('maintenance');params={'maintenanceId':m['id'],'expectedGeneration':m['generation']} if m else {};params.update(extra or {});return command(leader,dummy,kind,params=params)
 command(leader,dummy,'resume');maint('maintenance-request');wait(lambda:ds()['maintenance']['phase']=='maintenance');maint('maintenance-carrier-load',{'lp':'LP1'});maint('maintenance-end');maint('qualification-start');wait(lambda:ds()['maintenance']['phase']=='passed',240);wait(lambda:h.sql(f"SELECT count(*) FROM statistics_closures WHERE equipment_id='{dummy}' AND purpose='dummy'")=='4');maint('maintenance-carrier-restore')
 assert h.sql(f"SELECT count(*) FROM statistics_closures WHERE equipment_id='{dummy}' AND purpose='production'")=='0'
 # Restart source and bridge; counters and active qualification lock persist.
 prior=ds();subprocess.run(h.DC+['restart','simulator-05','bridge'],check=True,stdout=subprocess.DEVNULL);wait(lambda:ds()['communicationQuality']=='good' and ds()['paused']);assert ds()['maintenance']==prior['maintenance']
 assert all(leader.tool(i)['lots']==other[i] for i in other if i!=dummy)
 for period in ['today','24h','7d','30d']:
  r=leader.call('/equipment/statistics?period='+period);duration=sum(r['equipment'][0]['states'].values());assert all(sum(t['states'].values())==duration and len(t['timeline'])<=2017 for t in r['equipment']);assert sum(r['fleet']['states'].values())==duration*10
  for t in r['equipment']:
   counts=json.loads(h.sql(f"SELECT json_build_object('completed',count(*) FILTER(WHERE purpose='production' AND outcome='success'),'scrapped',count(*) FILTER(WHERE purpose='production' AND outcome='scrap'),'dummyCompleted',count(*) FILTER(WHERE purpose='dummy' AND outcome='success'),'dummyScrapped',count(*) FILTER(WHERE purpose='dummy' AND outcome='scrap')) FROM statistics_closures WHERE equipment_id='{t['equipmentId']}' AND closed_at>='{r['start']}' AND closed_at<'{r['end']}' AND NOT conflict"));assert all(t['production'][k]==v for k,v in counts.items())
 evidence['report']=r;evidence['dummy']=ds()['maintenance'];(ROOT/'.runtime/statistics-evidence.json').write_text(json.dumps(evidence,indent=2));print('PASS normal + supplement + scrap + 4 Dummy; role/read-only validation; 10-machine counts and intervals; restart/lock',flush=True)
if __name__=='__main__':run()
