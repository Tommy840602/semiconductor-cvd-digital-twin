#!/usr/bin/env python3
import json,pathlib,subprocess,time,urllib.request,http.cookiejar
root=pathlib.Path(__file__).resolve().parents[1];base='http://127.0.0.1:19080/api'
def sql(q):return subprocess.check_output(['docker','exec','cvd-kla-validation-postgres-1','psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
before=sql("SELECT count(*)||'|'||md5(string_agg(id||coalesce(result::text,''),',' ORDER BY id)) FROM metrology_runs")
for name,expected in [('kla-validation-disabled.json',False),('kla-validation.json',True)]:
 subprocess.run(['docker','compose','-f',str(root/'.runtime'/name),'up','-d','--no-deps','backend'],check=True,stdout=subprocess.DEVNULL)
 for n in range(120):
  try:
   urllib.request.urlopen(base+'/auth/csrf',timeout=2).read();break
  except Exception:
   if n%20==0:print('Waiting for backend',name,n,flush=True)
   time.sleep(1)
 else:raise AssertionError('backend not ready')
 op=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()));t=json.load(op.open(base+'/auth/csrf'))['token'];op.open(urllib.request.Request(base+'/auth/login',data=json.dumps({'username':'cvd.leader','password':(root/'.runtime/kla-test-secrets/leader_password').read_text().strip()}).encode(),headers={'Content-Type':'application/json','X-CSRF-Token':t})).read();assert json.load(op.open(base+'/metrology/station'))['enabled']==expected
 assert sql("SELECT count(*)||'|'||md5(string_agg(id||coalesce(result::text,''),',' ORDER BY id)) FROM metrology_runs")==before
 print('PASS dispatch enabled =',expected,'and all immutable results preserved',flush=True)
