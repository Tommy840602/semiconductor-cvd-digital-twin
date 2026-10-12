#!/usr/bin/env python3
import urllib.request,urllib.error,http.cookiejar,json,pathlib,uuid,subprocess
root=pathlib.Path(__file__).resolve().parents[1];base='http://127.0.0.1:19080/api';proof=json.loads((root/'.runtime/kla-stack-proof.json').read_text());id=proof['rows'][0]['id']
def session():return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def call(op,path,body=None,csrf=''):
 req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json','X-CSRF-Token':csrf})
 try:
  with op.open(req,timeout=10) as r:return r.status,json.load(r)
 except urllib.error.HTTPError as e:return e.code,e.read().decode()
def login(op,user,password):
 _,r=call(op,'/auth/csrf');code,r=call(op,'/auth/login',{'username':user,'password':password},r['token']);assert code==200;return r['csrf']
op=session();assert call(op,'/metrology/runs/'+id)[0]==401
csrf=login(op,'cvd.leader',(root/'.runtime/kla-test-secrets/leader_password').read_text().strip());assert call(op,'/metrology/runs/'+id+'/remeasure',{'requestId':str(uuid.uuid4())})[0]==403
password=uuid.uuid4().hex;username='kla.op.'+uuid.uuid4().hex[:8];assert call(op,'/users',{'username':username,'password':password,'role':'Operator','department':'EE'},csrf)[0] in [200,201]
op2=session();csrf2=login(op2,username,password);assert call(op2,'/metrology/runs/'+id)[0]==200
assert call(op2,'/metrology/runs/'+id+'/remeasure',{'requestId':str(uuid.uuid4())},csrf2)[0]==403
assert call(op2,'/metrology/runs/'+id+'/acknowledge',{},csrf2)[0]==403
print('PASS anonymous 401, CSRF 403, Operator read-only, privileged action rejection')
