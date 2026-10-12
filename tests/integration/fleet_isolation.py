from fleet import *
import subprocess
DC=['docker','compose','-f','.runtime/fleet-compose.json']
def publish(topic,body):subprocess.run(DC+['exec','-T','mqtt','mosquitto_pub','-t',topic,'-q','1','-s'],input=json.dumps(body),text=True,check=True)
def sql(q):return subprocess.check_output(DC+['exec','-T','postgres','psql','-U','cvd','-d','cvd','-Atc',q],text=True).strip()
def main():
 c=Client();id=IDS[8];other=IDS[9];c.tool(other,'/commands',dict(commandId=str(uuid.uuid4()),equipmentId=id,type='fault',targetId='PM1'),400);s=c.tool(id);old={**s,'version':0,'paused':not s['paused']};publish(f'cvd/{id}/snapshot',old)
 mismatch={**s,'equipmentId':other,'version':999999999};publish(f'cvd/{id}/snapshot',mismatch)
 wait(lambda:int(sql("select count(*) from dead_letter where body->>'payload'='equipment mismatch'"))>0)
 current=c.tool(id);assert current['version']>=s['version'];assert c.tool(other)['version']<999999999
 # Flushing only tool 09's cached snapshot must reconstruct from its own durable row.
 subprocess.run(DC+['exec','-T','redis','redis-cli','DEL',f'cvd:{id}:latest'],check=True,stdout=subprocess.DEVNULL)
 assert c.tool(id)['equipmentId']==id and c.tool(other)['equipmentId']==other
 # Stop only the gateway: other sources must remain fresh and advance.
 before=c.tool(other)['version'];subprocess.run(DC+['stop','gateway-09'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True)
 time.sleep(6)
 a=c.tool(id);b=c.tool(other);assert b['version']>before
 subprocess.run(DC+['start','gateway-09'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True)
 wait(lambda:c.tool(id)['version']>a['version'],60)
 pathlib.Path('.runtime/fleet-isolation.json').write_text(json.dumps({'oldSnapshotRejected':True,'mismatchedTopicRejected':True,'cacheReconstruction':True,'singleGatewayOutageIsolated':True},indent=2));print('PASS fleet replay, mismatch, cache and gateway isolation')
if __name__=='__main__':main()
