"""Additional isolated fault/restart checks. Uses tool 02 only."""
from exception_disposition import *
def run():
 leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip());ee=Client('disposition.ee',PASSWORD);pe=Client('disposition.pe',PASSWORD);id=IDS[1]
 command(leader,id,'resume');lot=id+'-OUTAGE-'+uuid.uuid4().hex[:6];command(leader,id,'start',lot=lot,params={'lp':'LP1','ll':'LL1'})
 p=wait(lambda:next((p for p in leader.tool(id)['chambers'] if p.get('processTelemetry') and p['processTelemetry']['thicknessNm']>1),None));command(leader,id,'fault',p['id']);auto=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['wafer_id']==p['waferId']),None));p=next(x for x in leader.tool(id)['chambers'] if x['id']==p['id']);t=p['processTelemetry']
 body=dict(incidentId=auto['incident_id'],pmId=p['id'],dispositionType='supplement',cause='模擬通訊中斷測試',correctiveAction='修復後保留原膜厚',verificationMethod='核对 checkpoint 數據',verificationResult='基線與身份一致',stepSeconds=[0,0,10,1],targetThicknessNm=t['thicknessNm']+t['rootDepositionRateNmPerSecond']*10)
 d=ee.tool(id,'/recoveries',body);rid=d['id'];rev=d['revision'];ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':rev});pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':rev});command(leader,id,'clear',p['id']);wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='executing');eid=ee.tool(id,'/recoveries/'+rid)['intents'][-1]['execution_id']
 subprocess.run(DC+['stop','simulator-02'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);subprocess.run(DC+['start','simulator-02'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 wait(lambda:leader.tool(id).get('communicationQuality')=='good' and leader.tool(id)['paused']);s=leader.tool(id);before=next(p for p in s['chambers'] if p['id']==body['pmId'])['processTelemetry'];time.sleep(2);after=next(p for p in leader.tool(id)['chambers'] if p['id']==body['pmId'])['processTelemetry'];assert before['thicknessNm']==after['thicknessNm'];assert s['dispositionHold'];command(leader,id,'resume',code=409)
 command(leader,id,'recovery-clock');subprocess.run(DC+['stop','gateway-02'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 wait(lambda:leader.tool(id).get('communicationQuality')!='good');leader.tool(id,'/release',code=409);version=leader.tool(IDS[2])['version'];time.sleep(12);assert leader.tool(IDS[2])['version']>version
 subprocess.run(DC+['start','gateway-02'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);wait(lambda:leader.tool(id).get('communicationQuality')=='good');d=wait(lambda:(lambda x:x if x['status']=='awaiting_result' else None)(ee.tool(id,'/recoveries/'+rid)),120)
 s=leader.tool(id);inc=s['incidents'][auto['incident_id']];assert inc['executionId']==eid and s['dispositionHold'];assert d['intents'][-1]['result']['result']['elapsedSeconds']==11
 # The result is invalidated by a new fault before confirmation.
 old=dict(revision=rev,executionId=eid,resultVersion=inc['resultVersion']);command(leader,id,'fault',p['id']);command(leader,id,'clear',p['id']);pe.tool(id,'/dispositions/'+rid+'/confirm',old,409);wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='failed')
 # Keep this safe unresolved incident for desktop/mobile EE form validation.
 d=ee.tool(id,'/recoveries/'+rid);(ROOT/'.runtime/disposition-outage-evidence.json').write_text(json.dumps(d,indent=2));print('PASS device restart preserves progress/hold; gateway outage preserves original execution; other tool samples; stale confirmation rejected',flush=True)
if __name__=='__main__':run()
