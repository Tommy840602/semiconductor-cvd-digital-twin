"""RBAC contract against isolated Docker services; no credentials printed."""
import os,json,uuid,pathlib,subprocess,time,asyncio,requests
from websockets.asyncio.client import connect
BASE=os.getenv('CVD_URL','http://localhost:19000')
DC=['docker','compose','-f',os.getenv('CVD_COMPOSE','.runtime/rbac-compose.json')]
def sql(q):return subprocess.check_output(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
def redis(*args):return subprocess.check_output(DC+['exec','-T','redis','redis-cli',*args],text=True).strip()
class Client:
 def __init__(self):
  self.s=requests.Session();r=self.s.get(BASE+'/api/auth/csrf');assert r.status_code==200,r.status_code;self.csrf=r.json()['token']
 def call(self,path,body=None,code=200,csrf=True):
  r=self.s.request('GET' if body is None else 'POST',BASE+'/api'+path,json=body,headers={'X-CSRF-Token':self.csrf} if csrf else {},timeout=15)
  assert r.status_code==code,(path,r.status_code,r.text[:180]);return r.json()
 def login(self,name,password):
  self.csrf=self.call('/auth/csrf')['token']
  r=self.call('/auth/login',dict(username=name,password=password));self.csrf=r['csrf'];return r['user']
 def cookie(self):return 'CVD_SESSION='+self.s.cookies.get('CVD_SESSION')
def main():
 leader=Client();assert leader.call('/auth/me',code=401)
 for p in ['/equipment','/history','/commands','/telemetry','/audit']:
  assert requests.get(BASE+'/api'+p).status_code==401,p
 pwd=pathlib.Path(os.getenv('CVD_PASSWORD_FILE','.runtime/secrets/test-leader-password')).read_text().strip()
 leader.call('/auth/login',dict(username='cvd.leader',password=pwd),code=403,csrf=False)
 leader.call('/auth/login',dict(username='cvd.leader',password='incorrect-password'),code=401)
 leader.call('/auth/login',dict(username='missing-user',password='incorrect-password'),code=401)
 assert leader.login('cvd.leader',pwd)['role']=='Leader'
 assert leader.call('/auth/me')['username']=='cvd.leader'
 assert 'HttpOnly' in requests.get(BASE+'/api/auth/csrf').headers['Set-Cookie']
 suffix=uuid.uuid4().hex[:8];password=uuid.uuid4().hex
 people={'Leader':leader};names={}
 for role in ['Operator','Engineer']:
  name=role.lower()+'.'+suffix;names[role]=name
  u=leader.call('/users',dict(username=name,password=password,role=role));assert 'password' not in json.dumps(u)
  leader.call('/users',dict(username=name,password=password,role=role),code=409)
  c=Client();assert c.login(name,password)['role']==role;people[role]=c
 for role in ['Leader','Unknown']:
  leader.call('/users',dict(username='denied.'+suffix,password=password,role=role),code=400)
 for role in ['Operator','Engineer']:
  people[role].call('/users',dict(username='denied.'+suffix,password=password,role='Operator'),code=403)
 before=int(sql('select count(*) from commands'))
 denied=['configure','fault','clear','recovery-clock','scrap','resume','unknown']
 for kind in denied:
  people['Operator'].call('/commands',dict(commandId=str(uuid.uuid4()),type=kind,actor={'username':'cvd.leader'},role='Leader'),code=403)
 assert int(sql('select count(*) from commands'))==before
 people['Engineer'].call('/commands',dict(commandId=str(uuid.uuid4()),type='unknown'),code=403)
 # Expired commands exercise acceptance/RBAC without modifying the equipment during matrix checks.
 ids=[]
 for role,c in people.items():
  kinds=['start','pause','stop']+([] if role=='Operator' else denied[:-1])
  for kind in kinds:
   cid=str(uuid.uuid4());ids.append(cid)
   r=c.call('/commands',dict(commandId=cid,type=kind,expiresAt='2000-01-01T00:00:00Z',actor={'username':'spoof'},parameters={}))
   assert r['actor']==('cvd.leader' if role=='Leader' else names[role])
 cid=ids[0];people['Engineer'].call('/commands',dict(commandId=cid,type='start',expiresAt='2000-01-01T00:00:00Z'))
 assert leader.call('/commands/'+cid)['actor']=='cvd.leader'
 audits=leader.call('/audit');assert any(a['outcome']=='denied' for a in audits);assert password not in json.dumps(audits)
 assert sql("select count(*) from app_users where password_hash not like '$2%'")=='0'
 assert len(leader.call('/history'))>0
 async def sockets():
  url=BASE.replace('http','ws',1)+'/ws'
  for headers,origin in [({},BASE),({'Cookie':leader.cookie()},'https://evil.invalid')]:
   try:
    async with connect(url,additional_headers=headers,origin=origin):raise AssertionError('unauthorized socket')
   except Exception as e:
    if isinstance(e,AssertionError):raise
  c=people['Operator']
  async with connect(url,additional_headers={'Cookie':c.cookie()},origin=BASE) as ws:
   await asyncio.wait_for(ws.recv(),10);c.call('/auth/logout',{})
   await asyncio.wait_for(ws.wait_closed(),5);assert ws.close_code==1008
  c.call('/equipment',code=401)
  c.login(names['Operator'],password)
  sid=c.s.cookies.get('CVD_SESSION')
  async with connect(url,additional_headers={'Cookie':c.cookie()},origin=BASE) as ws:
   await asyncio.wait_for(ws.recv(),10)
   redis('EXPIRE','cvd:session:'+sid,'2')
   for _ in range(2):c.call('/equipment');time.sleep(.4)
   await asyncio.wait_for(ws.wait_closed(),6);assert ws.close_code==1008
  c.call('/equipment',code=401)
 asyncio.run(sockets())
 # Repeated invalid attempts are bounded.
 bad=Client()
 for i in range(10):bad.call('/auth/login',dict(username='rate.'+suffix,password='bad'),code=401)
 bad.call('/auth/login',dict(username='rate.'+suffix,password='bad'),code=429)
 pathlib.Path('.runtime/rbac-browser-users.json').write_text(json.dumps({'names':names,'password':password}))
 os.chmod('.runtime/rbac-browser-users.json',0o600)
 print('PASS RBAC: authentication, CSRF, all roles/commands, account creation, audit, dedup, websocket logout/expiry, throttling, old history')
if __name__=='__main__':main()
