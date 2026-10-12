#!/usr/bin/env python3
"""Fail closed before restoring pre-maintenance simulator code against checkpoints."""
import argparse,json,pathlib,sys
p=argparse.ArgumentParser();p.add_argument('state_directory',type=pathlib.Path);a=p.parse_args()
files=list(a.state_directory.glob('*.json'))
if len(files)!=10:sys.exit('REFUSED: require all ten equipment checkpoints')
seen=set()
for file in files:
 s=json.loads(file.read_text());seen.add(s['equipmentId']);m=s.get('maintenance')
 if m and (m.get('phase')!='released' or m.get('loaded')):sys.exit('REFUSED: active maintenance on '+s['equipmentId'])
 if any(l.get('purpose')=='dummy' for l in s.get('lots',[])):sys.exit('REFUSED: checkpoint contains Dummy lots unsupported by old code; restore complete pre-upgrade backup')
 if any(l.get('parked') and l.get('purpose')!='dummy' for l in s.get('lots',[])):sys.exit('REFUSED: parked production FOUP on '+s['equipmentId'])
if seen!={f'TEL-CVDTIN-08A-{i:02}' for i in range(1,11)}:sys.exit('REFUSED: incomplete equipment identities')
print('PASS: no active maintenance or parked production FOUP. Restore a consistent pre-upgrade backup for old simulator code; never reuse new checkpoints with Dummy lots.')
