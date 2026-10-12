"""Read-only API checks plus controlled isolated Redis/backend restarts."""
from rbac import *
def run():
 c=Client();c.login('cvd.leader',pathlib.Path('.runtime/secrets/test-leader-password').read_text())
 before=sql("select md5(string_agg(id::text||username||role||password_hash,',' order by id)) from app_users")
 async def outage():
  async with connect(BASE.replace('http','ws',1)+'/ws',additional_headers={'Cookie':c.cookie()},origin=BASE) as ws:
   await asyncio.wait_for(ws.recv(),10)
   subprocess.run(DC+['stop','redis'],check=True,stdout=subprocess.DEVNULL)
   try:
    c.call('/equipment',code=503)
    await asyncio.wait_for(ws.wait_closed(),15);assert ws.close_code==1008
   finally:subprocess.run(DC+['start','redis'],check=True,stdout=subprocess.DEVNULL)
 asyncio.run(outage())
 for _ in range(30):
  try:c.call('/equipment');break
  except:time.sleep(1)
 else:raise AssertionError('Redis recovery timeout')
 subprocess.run(DC+['restart','backend'],check=True,stdout=subprocess.DEVNULL)
 for _ in range(60):
  try:
   r=c.call('/auth/me');assert r['role']=='Leader';break
  except:time.sleep(1)
 else:raise AssertionError('Backend recovery timeout')
 assert before==sql("select md5(string_agg(id::text||username||role||password_hash,',' order by id)) from app_users")
 print('PASS recovery: Redis outage denied API/closed WS, Redis session recovered, backend restart preserved all accounts/passwords')
if __name__=='__main__':run()
