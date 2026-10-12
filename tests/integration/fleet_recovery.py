"""EE/PE and cross-tool recovery validation on the isolated fleet."""
from fleet import *
import subprocess

def main():
 step=8 if os.environ.get('CVD_TEST_RECOVERY_OUTAGE') else 3
 leader=Client();password='Fleet-Recovery-Test-2026!'
 for name,role,dept in [('fleet.ee','Engineer','EE'),('fleet.pe','Engineer','PE'),('fleet.op','Operator','EE')]:
  if not any(u['username']==name for u in leader.call('/users')):leader.call('/users',dict(username=name,password=password,role=role,department=dept))
 ee,pe,op=[Client(n,password) for n in ['fleet.ee','fleet.pe','fleet.op']]
 id=IDS[0];other=IDS[1]
 op.tool(id,'/recipes',{},403)
 r=leader.tool(id,'/recipes');out=leader.tool(id,'/recipes',{'expectedVersion':r['currentVersion'],'settings':{'stepSeconds':dict(preheat=1,stabilize=1,deposition=20,purge=1)}})
 wait(lambda:leader.tool(id,'/commands/'+out['commandId'])['status']=='succeeded')
 assert leader.tool(other)['recipeSteps']==[1,1,4,1], 'Copied recipe must remain independent'
 lot=id+'-RECOVERY-'+uuid.uuid4().hex[:8];command(leader,id,'start',lot,params={'lp':'LP1','ll':'LL1'});command(leader,id,'resume')
 p=wait(lambda:next((p for p in leader.tool(id)['chambers'] if p.get('waferId') and p.get('processTelemetry',{}).get('thicknessNm',0)>1),None))
 command(leader,id,'fault',target=p['id']);p=next(p2 for p2 in leader.tool(id)['chambers'] if p2['id']==p['id'])
 t=p['processTelemetry'];body=dict(pmId=p['id'],stepSeconds=[0,0,step,1],targetThicknessNm=t['thicknessNm']+t['rootDepositionRateNmPerSecond']*step,troubleshooting='EE 已排除模擬故障，精確追加沉積與吹掃')
 d=ee.tool(id,'/recoveries',body);rid=d['id'];baseline=d['versions'][0]['prescription']['baselineThicknessNm']
 ee.tool(other,'/recoveries/'+rid,code=404)
 ee.tool(id,'/recoveries/'+rid+'/submit',{'revision':1})
 ee.tool(id,'/recoveries/'+rid+'/approve',{'revision':1},403)
 pe.tool(other,'/recoveries/'+rid+'/approve',{'revision':1},404)
 d=pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':1});eid=d['intents'][0]['execution_id']
 d=pe.tool(id,'/recoveries/'+rid+'/approve',{'revision':1});assert len(d['intents'])==1
 assert eid not in (leader.tool(other).get('recoveries') or {})
 command(leader,id,'clear',target=p['id']);command(leader,id,'recovery-clock')
 if os.environ.get('CVD_TEST_RECOVERY_OUTAGE'):
  wait(lambda:(leader.tool(id).get('recoveries') or {}).get(eid,{}).get('status')=='executing',30)
  dc=['docker','compose','-f','.runtime/secs-compose.json'];subprocess.run(dc+['stop','gateway'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  before=leader.tool(other)['version'];time.sleep(6);assert leader.tool(other)['version']>before
  subprocess.run(dc+['start','gateway'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

 d=wait(lambda:(lambda d:d if d['status'] in ('completed','failed') else None)(ee.tool(id,'/recoveries/'+rid)),60)
 assert d['status']=='completed',d
 intent=d['intents'][0]['result'];assert intent['result']['baselineThicknessNm']==baseline
 assert intent['result']['thicknessNm']>=baseline
 assert intent['result']['stepSeconds']==[0,0,step,1]
 assert eid not in (leader.tool(other).get('recoveries') or {})
 pathlib.Path('.runtime/secs-recovery.json' if ':21000/' in BASE else '.runtime/fleet-recovery.json').write_text(json.dumps({'request':rid,'execution':eid,'status':d['status'],'baseline':baseline,'after':intent['result']['thicknessNm'],'crossToolRejected':True,'duplicateApprovalSingleIntent':True},indent=2))
 print('PASS: EE/PE recovery remains bound to original tool and wafer',flush=True)
if __name__=='__main__':main()
