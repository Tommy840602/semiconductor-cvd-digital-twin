"""Exercise the real HTTP -> MQTT -> OPC UA -> Kafka -> storage path."""
import json, time, urllib.request, urllib.error, http.cookiejar, uuid, os, pathlib
BASE=os.getenv('CVD_URL','http://localhost:18080')
SESSION=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
CSRF=None
def raw(path,body=None):
    req=urllib.request.Request(BASE+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json','X-CSRF-Token':CSRF or ''})
    with SESSION.open(req,timeout=10) as response:return json.load(response)
def request(path,body=None):
    global CSRF
    if CSRF is None:
        CSRF=raw('/auth/csrf')['token']
        secret=pathlib.Path(os.getenv('CVD_PASSWORD_FILE','.runtime/secrets/leader-password')).read_text().strip()
        result=raw('/auth/login',dict(username=os.getenv('CVD_USERNAME','cvd.leader'),password=secret));CSRF=result['csrf']
    return raw(path,body)
def wait(fn,timeout=60):
    end=time.time()+timeout
    while time.time()<end:
        value=fn()
        if value:return value
        time.sleep(1)
    raise AssertionError('Timed out waiting for state')
def command(kind,target='',lot='',parameters=None):
    id=str(uuid.uuid4());request('/commands',dict(commandId=id,type=kind,targetId=target,lotId=lot,parameters=parameters or {}))
    result=wait(lambda: (r if (r:=request('/commands/'+id))['status']!='received' else None),40)
    assert result['status']=='succeeded',result
    return id

def normal():
    command('resume')
    seconds=int(os.getenv('CVD_PROCESS_SECONDS','30'))
    command('configure',parameters={'processSeconds':seconds})
    names=['VERIFY-'+uuid.uuid4().hex[:6] for _ in range(3)]
    for i,name in enumerate(names,1):command('start',lot=name,parameters={'lp':f'LP{i}','ll':f'LL{i}'})
    peak=0;mixed=False;end=time.time()+2400
    while time.time()<end:
        s=request('/equipment');lots=[l for l in s['lots'] if l['id'] in names];assert sum(len(l['wafers']) for l in lots)==75
        for lock in s['locks']:
            occupancy=sum(w['location']==lock['id'] for l in lots for w in l['wafers'])
            incoming=sum(bool(r['waferId']) and r['to']==lock['id'] for r in [s['atm'],s['vac']])
            assert occupancy+incoming<=1,(lock,occupancy,incoming)
        positions=[w['id'] for l in lots for w in l['wafers']];assert len(set(positions))==75
        running=[p for p in s['chambers'] if p['waferId']];peak=max(peak,len(running))
        batches={p['waferId'].rsplit('-W',1)[0] for p in running};mixed=mixed or len(batches)==3
        if all(l['phase']=='complete' for l in lots):
            for l in lots:assert all(w['location']==l['lp'] and w['result']=='success' for w in l['wafers'])
            assert peak>=(4 if seconds>=30 else 3) and mixed,(peak,mixed)
            print('PASS normal: 75 wafers, concurrent round-robin lots, shared PMs, original LP return',flush=True);return
        time.sleep(1)
    raise AssertionError('normal batch timeout')

def fault(action):
    name='FAULT-'+uuid.uuid4().hex[:6];command('start',lot=name,parameters={'lp':'LP1','ll':'LL2'})
    state=wait(lambda: (s if (s:=request('/equipment'))['chambers'][0]['waferId'] else None),90)
    wafer=state['chambers'][0]['waferId'];command('fault','PM1',name)
    time.sleep(8);s=request('/equipment');assert next(l for l in s['lots'] if l['id']==name)['paused']
    assert next(w for l in s['lots'] for w in l['wafers'] if w['id']==wafer)['result']=='exception'
    command('clear','PM1',name);command(action,'PM1',name)
    wait(lambda:next(w for l in request('/equipment')['lots'] for w in l['wafers'] if w['id']==wafer)['location']=='LP1',120)
    assert next(l for l in request('/equipment')['lots'] if l['id']==name)['paused']
    command('resume',lot=name)
    l=wait(lambda: (l if (l:=next(l for l in request('/equipment')['lots'] if l['id']==name))['phase']=='complete' else None),900)
    scraps=sum(w['result']=='scrap' for w in l['wafers']);assert scraps==(1 if action=='scrap' else 0)
    assert all(w['location']=='LP1' for w in l['wafers'])
    history=request('/wafers/'+wafer+'/history');assert any(e['kind']==action for e in history)
    print('PASS fault '+action+': pause, disposition, manual resume, history',flush=True)

if __name__=='__main__':
    normal();fault('scrap')
    assert request('/telemetry'), 'no TimescaleDB samples'
    print('PASS all end-to-end scenarios',flush=True)
