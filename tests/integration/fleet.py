"""Fleet acceptance against isolated cvd-fleet-test only; never production."""
import requests,time,uuid,pathlib,json,concurrent.futures,os
BASE=os.environ.get('CVD_TEST_API','http://127.0.0.1:20000/api')
assert BASE in ('http://127.0.0.1:20000/api','http://127.0.0.1:21000/api'), 'isolated test endpoints only'
IDS=[f'TEL-CVDTIN-08A-{n:02}' for n in range(1,11)]
def wait(fn,seconds=90):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  r=fn()
  if r:return r
  time.sleep(.5)
 raise AssertionError('timed out')
class Client:
 def __init__(self,name='cvd.leader',password=None):
  self.s=requests.Session();self.csrf=self.call('/auth/csrf')['token'];r=self.call('/auth/login',{'username':name,'password':password or pathlib.Path(os.environ.get('CVD_TEST_PASSWORD_FILE','.runtime/secrets/fleet-test-password')).read_text().strip()});self.csrf=r['csrf']
 def call(self,path,body=None,code=200):
  r=self.s.get(BASE+path,timeout=15) if body is None else self.s.post(BASE+path,json=body,headers={'X-CSRF-Token':self.csrf},timeout=15)
  assert r.status_code==code,(path,r.status_code,r.text[:500]);return r.json() if r.content else None
 def tool(self,id,path='',body=None,code=200):return self.call('/equipment/'+id+path,body,code)
def command(c,id,kind,lot='',target='',params=None):
 cid=str(uuid.uuid4());body=dict(commandId=cid,type=kind,lotId=lot,targetId=target,parameters=params or {})
 c.tool(id,'/commands',body)
 r=wait(lambda:(lambda r:r if r['status'] not in ('received','accepted') else None)(c.tool(id,'/commands/'+cid)))
 assert r['status']=='succeeded',r
 return cid,body
def main():
 c=Client();fleet=wait(lambda:(lambda f:f if all(x['sourceTime'] for x in f) else None)(c.call('/equipment')))
 assert [m['equipmentId'] for m in fleet]==IDS
 c.call('/commands',dict(commandId=str(uuid.uuid4()),type='resume'),404)
 c.tool('TEL-CVDTIN-08A-11',code=400)
 # Independent recipe versions and conflict handling.
 recipes=[c.tool(id,'/recipes') for id in IDS]
 p={'preheat':1,'stabilize':1,'deposition':4,'purge':1}
 for id,r in zip(IDS,recipes):
  out=c.tool(id,'/recipes',{'expectedVersion':r['currentVersion'],'settings':{'stepSeconds':p}})
  wait(lambda:c.tool(id,'/commands/'+out['commandId'])['status']=='succeeded')
 first=c.tool(IDS[0],'/recipes');second=c.tool(IDS[1],'/recipes')
 copied=c.tool(IDS[1],'/recipes',{'expectedVersion':second['currentVersion'],'sourceEquipment':IDS[0],'sourceVersion':first['currentVersion']})
 c.tool(IDS[1],'/recipes',{'expectedVersion':second['currentVersion'],'sourceEquipment':IDS[0],'sourceVersion':first['currentVersion']},409)
 assert copied['recipe']['versions'][0]['source_version']==first['currentVersion']
 print('Recipe versions, copy and conflict checks passed',flush=True)
 # 10 tools x 3 lots; real OPC UA, MQTT, Kafka pipeline.
 lots={}
 for id in IDS:
  command(c,id,'resume');lots[id]=[]
  for n in range(1,4):
   lot=id+'-LOT-'+uuid.uuid4().hex[:12];lots[id].append(lot);command(c,id,'start',lot,params={'lp':f'LP{n}','ll':f'LL{n}'})
 print('Started all 30 lots on 10 tools',flush=True)
 cid,body=command(c,IDS[0],'fault',target='PM1')
 assert not c.tool(IDS[1])['chambers'][0]['fault']
 c.tool(IDS[1],'/commands/'+cid,code=404)
 c.tool(IDS[1],'/commands',body,409)
 c.tool(IDS[1],'/commands',dict(commandId=str(uuid.uuid4()),type='pause',lotId=lots[IDS[0]][0]),404)
 # Clear/recover or scrap just the isolated fault wafer to complete all lots.
 state=c.tool(IDS[0]);pm=state['chambers'][0]
 command(c,IDS[0],'clear',target='PM1')
 if pm.get('waferId'):
  command(c,IDS[0],'scrap',target='PM1')
  wait(lambda:all(w['location']==l['lp'] for l in c.tool(IDS[0])['lots'] for w in l['wafers'] if w['result']=='scrap'))
 command(c,IDS[0],'resume')
 for lot in lots[IDS[0]]:command(c,IDS[0],'resume',lot)
 progress=[-1]
 def complete():
  states=[c.tool(id) for id in IDS]
  done=sum(all(l['phase']=='complete' for l in s['lots']) for s in states)
  if done!=progress[0]:print('Completed tools:',done,flush=True);progress[0]=done
  if done==10:return states
  return None
 states=wait(complete,600)
 for id,s in zip(IDS,states):
  wafers=[w for l in s['lots'] for w in l['wafers']];assert len(wafers)==75 and len({w['id'] for w in wafers})==75
  assert all(w['location']==l['lp'] for l in s['lots'] for w in l['wafers'])
  rows=c.tool(id,'/history');assert rows and all(e['equipmentId']==id for e in rows)
  assert c.tool(id,'/telemetry?metric=temperature&pmId=PM1')['points']
 pathlib.Path('.runtime/fleet-acceptance.json').write_text(json.dumps({'equipmentCount':10,'waferCount':750,'independentRecipes':True,'crossToolRejected':True,'allLotsCompleted':True},indent=2))
 print('PASS: 10 tools, 750 wafers, isolated control/history and recipe versions',flush=True)
if __name__=='__main__':main()
