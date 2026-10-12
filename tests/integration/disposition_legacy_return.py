"""Rehearse old direct-scrap migration on a disposable tool 04 checkpoint."""
from exception_disposition import *
def run():
 id=IDS[3];service='simulator-04';leader=Client('cvd.leader',(ROOT/'.runtime/secrets/disposition-leader-password').read_text().strip());ee=Client('disposition.ee',PASSWORD);pe=Client('disposition.pe',PASSWORD)
 assert not leader.tool(id)['lots'],'fixture requires unused isolated tool 04'
 subprocess.run(DC+['stop',service],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 container=subprocess.check_output(DC+['ps','-a','-q',service],text=True).strip();temp=ROOT/'.runtime/legacy-return-fixture.json';subprocess.run(['docker','cp',container+':/data/state.json',str(temp)],check=True,stdout=subprocess.DEVNULL)
 state=json.loads(temp.read_text());lot=id+'-LEGACY';wafer=lot+'-W01';state.update(modelVersion=5,dispositionMigrationVersion=0,dispositionHold=False,dispatchPaused=True,paused=True,incidents={},recoveries={},releases={},authorizations={})
 state['lots']=[dict(id=lot,lp='LP1',ll='LL1',phase='processing',paused=True,order=1,wafers=[dict(id=lot+f'-W{n:02}',lotId=lot,slot=n,location='LP1',result='scrap' if n==1 else 'pending',attempts=1 if n==1 else 0,exceptionVersion=1 if n==1 else 0) for n in range(1,26)])]
 state['locks'][0].update(lotId=lot,phase='atmosphere',remaining=0);temp.write_text(json.dumps(state));subprocess.run(['docker','cp',str(temp),container+':/data/state.json'],check=True,stdout=subprocess.DEVNULL);subprocess.run(DC+['start',service],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 wait(lambda:leader.tool(id).get('dispositionMigrationVersion')==1 and leader.tool(id)['dispositionHold']);auto=wait(lambda:next((r for r in ee.tool(id,'/recoveries') if r['wafer_id']==wafer),None));assert auto['owner_id'] is None
 command(leader,id,'resume',code=409)
 body=dict(incidentId=auto['incident_id'],pmId='',dispositionType='legacy_return',cause='舊版已報廢但批次未放行',correctiveAction='核對既有異常與報廢紀錄',verificationMethod='核對原LP與晶圓槽位',verificationResult='已在LP1原第1槽且保持報廢')
 ee.tool(id,'/recoveries',{**body,'dispositionType':'supplement'},409)
 d=ee.tool(id,'/recoveries',body);rid=d['id'];rev=d['revision'];assert d['versions'][-1]['prescription']['baselineThicknessNm'] is None
 ee.tool(id,'/recoveries/'+rid+'/submit',dict(revision=rev));pe.tool(id,'/recoveries/'+rid+'/approve',dict(revision=rev));command(leader,id,'recovery-clock');wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='awaiting_result');s=leader.tool(id);w=s['lots'][0]['wafers'][0];assert (w['location'],w['result'],w['attempts'])==('LP1','scrap',1)
 inc=s['incidents'][auto['incident_id']];pe.tool(id,'/dispositions/'+rid+'/confirm',dict(revision=rev,executionId=inc['executionId'],resultVersion=inc['resultVersion']));wait(lambda:ee.tool(id,'/recoveries/'+rid)['status']=='resolved');view=wait(lambda:(lambda v:v if v['eligible'] else None)(leader.tool(id,'/release')));leader.tool(id,'/release',dict(generation=view['generation']));wait(lambda:not leader.tool(id)['dispositionHold']);
 d=ee.tool(id,'/recoveries/'+rid);(ROOT/'.runtime/disposition-legacy-return-evidence.json').write_text(json.dumps(d,indent=2));print('PASS legacy direct scrap: no reload, no growth, no fake baseline/signatures; EE/PE review then independent Leader release',flush=True)
if __name__=='__main__':run()
