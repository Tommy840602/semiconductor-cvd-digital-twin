#!/usr/bin/env python3
"""Prepare offline tool state copies. Never overwrites an existing destination."""
import argparse,json,pathlib,copy
p=argparse.ArgumentParser();p.add_argument('source');p.add_argument('destination');a=p.parse_args()
s=json.loads(pathlib.Path(a.source).read_text());dest=pathlib.Path(a.destination);dest.mkdir(parents=True,exist_ok=True)
for n in range(1,11):
 id=f'TEL-CVDTIN-08A-{n:02}';out=dest/(id+'.json')
 if out.exists():
  assert json.loads(out.read_text())['equipmentId']==id
  continue
 if n==1:t=copy.deepcopy(s)
 else:
  t={'modelVersion':5,'modelSeed':s.get('modelSeed',20260915)+n,'version':0,'epoch':id+'-initial','sequence':0,'tick':0,'lots':[],'locks':[{'id':f'LL{i}','phase':'empty','pressure':1013} for i in range(1,4)],'chambers':[{'id':f'PM{i}','temperature':25} for i in range(1,5)],'atm':{'id':'ATM'},'vac':{'id':'VAC'},'commands':{},'events':[],'recoveries':{},'processSeconds':s['processSeconds']}
  if s.get('recipeSteps') is not None:t['recipeSteps']=s['recipeSteps']
 t['equipmentId']=id;t['recipeVersion']=id+'-recipe-1';t['paused']=True
 out.write_text(json.dumps(t));out.chmod(0o600)
print('Prepared ten independent states; existing files preserved')
