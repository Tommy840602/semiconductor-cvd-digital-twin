#!/usr/bin/env python3
"""Create a consistent maintenance backup of cvd-twin. Leaves services stopped.
Run only as part of an authorized maintenance cutover. No volumes are deleted.
"""
import argparse,datetime,hashlib,json,pathlib,shutil,subprocess,tarfile
p=argparse.ArgumentParser();p.add_argument('directory');a=p.parse_args();root=pathlib.Path(__file__).resolve().parents[1];out=pathlib.Path(a.directory).resolve()
if out.exists():raise SystemExit('Backup destination must be new')
out.mkdir(parents=True,mode=0o700)
def run(args,**kw):return subprocess.run(args,check=True,cwd=root,**kw)
def capture(args):return subprocess.check_output(args,cwd=root,text=True)
dc=['docker','compose']  # Honors the active .env COMPOSE_FILE after cutover.
c=json.loads(capture(dc+['config','--format','json']));assert c['name']=='cvd-twin'
stamp=datetime.datetime.now().strftime('%Y%m%d-%H%M%S')
for name,s in c['services'].items():
 built='build' in s;s.pop('build',None)
 if built or s.get('image','').startswith(('cvd-twin-','cvd-secs-')):
  container=capture(dc+['ps','-a','-q',name]).strip();assert container,'Missing deployed container: '+name
  deployed=capture(['docker','inspect',container,'--format','{{.Image}}']).strip()
  tag='cvd-runtime-backup-'+stamp+'-'+name;run(['docker','tag',deployed,tag]);s['image']=tag
 for i,v in enumerate(s.get('volumes',[])):
  if v['type']=='bind' and pathlib.Path(v['source']).is_file():
   dest=out/'bindings'/(name+'-'+str(i));dest.parent.mkdir(exist_ok=True);shutil.copyfile(v['source'],dest);v['source']=str(dest)
for name,s in c.get('secrets',{}).items():
 dest=out/'secrets'/name;dest.parent.mkdir(exist_ok=True);shutil.copyfile(s['file'],dest);s['file']=str(dest)
(out/'compose.json').write_text(json.dumps(c,indent=2))
producers=[n for n in c['services'] if n in ('frontend','backend','bridge') or n.startswith(('gateway','simulator'))]
run(dc+['stop']+producers)
for service in ('postgres','timescaledb'):
 with (out/(service+'.dump')).open('wb') as f:run(dc+['exec','-T',service,'pg_dump','-U','cvd','-d','cvd','-Fc'],stdout=f)
run(dc+['stop'])
manifest={}
for name,v in c['volumes'].items():
 print('Backing up',name,flush=True)
 target=out/(name+'.tar.gz')
 run(['docker','run','--rm','-v',v['name']+':/data:ro','-v',str(out)+':/backup','alpine:3.22','tar','czf','/backup/'+target.name,'-C','/data','.'])
 digest=hashlib.sha256()
 with target.open('rb') as f:
  for b in iter(lambda:f.read(4*1024*1024),b''):digest.update(b)
 manifest[name]={'volume':v['name'],'archive':target.name,'bytes':target.stat().st_size,'sha256':digest.hexdigest()}
 if name.startswith('sim-'):
  with tarfile.open(target) as t:state=json.load(t.extractfile('./state.json'))
  folder=out/'states-original';folder.mkdir(exist_ok=True);(folder/(state['equipmentId']+'.json')).write_text(json.dumps(state))
(out/'manifest.json').write_text(json.dumps(manifest,indent=2));print('Consistent backup complete; production remains stopped:',out,flush=True)
