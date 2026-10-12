"""Validate completed run history against final persisted telemetry."""
import json,subprocess
DC=['docker','compose','-f','.runtime/process-compose.json']
def sql(service,q):return subprocess.check_output(DC+['exec','-T',service,'psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
def rows(service,q):return [json.loads(x) for x in sql(service,q).splitlines()]
events=rows('postgres',"SELECT body FROM history WHERE kind IN ('process-result','process-phase-changed') AND time<now()-interval '3 seconds' ORDER BY time,(body->>'sequence')::bigint,id")
samples=rows('timescaledb',"SELECT json_build_object('run',p->>'processRunId','phases',jsonb_agg(DISTINCT p#>'{processTelemetry,phase}'),'final',jsonb_agg(p->'processTelemetry') FILTER (WHERE p#>>'{processTelemetry,elapsedSeconds}'=p#>>'{processTelemetry,durationSeconds}')) FROM telemetry CROSS JOIN LATERAL jsonb_array_elements(body#>'{payload,chambers}') p WHERE p->'processTelemetry' IS NOT NULL GROUP BY p->>'processRunId'")
byrun={s['run']:s for s in samples};count=0
for e in events:
 if e['kind']!='process-result' or not e['payload']['complete']:continue
 r=e['payload'];run=r['processRunId'];s=byrun[run]
 assert any(p['thicknessNm']==r['result']['thicknessNm'] for p in s['final'] or []),run
 assert {'preheat','stabilize','deposition','purge'} <= set(s['phases']),run
 phases=[x['payload']['phase'] for x in events if x['kind']=='process-phase-changed' and x['payload']['processRunId']==run]
 assert phases==['preheat','stabilize','deposition','purge','complete'],(run,phases)
 count+=1
assert count>0
print('PASS',count,'completed runs: four phase samples, five ordered events, final thickness equal after PM pickup')
