"""Check persisted partial attempts remain distinct from completed rework."""
import json,subprocess
DC=['docker','compose','-f','.runtime/process-compose.json']
raw=subprocess.check_output(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-Atc',"SELECT body FROM history WHERE kind='process-result' ORDER BY time"],text=True)
events=[json.loads(x) for x in raw.splitlines()]
partial=[e for e in events if not e['payload']['complete']]
assert {e['payload']['reason'] for e in partial}=={'reworked','scrapped'},partial
for e in partial:
 p=e['payload'];assert p['result']['quality']=='not-evaluated'
 if p['reason']=='reworked':
  completed=[x for x in events if x['waferId']==e['waferId'] and x['payload']['complete']]
  assert len(completed)==1 and completed[0]['payload']['processRunId']!=p['processRunId']
 else:assert not any(x['waferId']==e['waferId'] and x['payload']['complete'] for x in events)
assert len({e['payload']['processRunId'] for e in events})==len(events)
print('PASS incomplete rework/scrap results, distinct retry run, no fabricated successful scrap or duplicate result')
