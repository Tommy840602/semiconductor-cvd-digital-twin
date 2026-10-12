from pathlib import Path
source=Path('tests/integration/dummy_kla_approval.py').read_text();exec(source[:source.index("command('resume')")])
maint('maintenance-request');wait(lambda:state()['maintenance']['phase']=='maintenance');maint('maintenance-end');maint('maintenance-carrier-load',{'lp':'LP2'})
subprocess.run(compose+['stop','metrology'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
maint('qualification-start');wait(lambda:state()['maintenance']['phase']=='passed',300)
qpath='/equipment/'+eq+'/maintenance/qualification';q=api(qpath);assert not q['canConfirm'];assert state()['dispatchPaused'];print('PASS interrupted KLA blocks qualification',flush=True)
subprocess.run(compose+['start','metrology'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
q=wait(lambda:(q if q['canConfirm'] else False) if (q:=api(qpath)) else False,300)
oldrun=q['evidence']['runId'];api(qpath,{'requestId':str(uuid.uuid4()),'decision':'reject','reason':'test complete new qualification','digest':q['digest']});assert state()['dispatchPaused']
maint('qualification-start');wait(lambda:state()['maintenance']['phase']=='passed',300)
q=wait(lambda:(q if q['canConfirm'] else False) if (q:=api(qpath)) else False,300);assert q['evidence']['runId']!=oldrun
Path('.runtime/dummy-ui-fixture.json').write_text(json.dumps({'state':state(),'qualification':q}))
q=api(qpath,{'requestId':str(uuid.uuid4()),'decision':'confirm','reason':'new run passed','digest':q['digest']});assert q['confirmed'];assert state()['dispatchPaused'];assert state()['maintenance']['loaded']
maint('maintenance-carrier-restore');maint('resume');assert state()['maintenance']['phase']=='released'
Path('.runtime/dummy-flow-second-proof.json').write_text(json.dumps(q,indent=2));print('PASS KLA restart, PE reject, new four-wafer qualification, confirm before restore and manual resume',flush=True)
