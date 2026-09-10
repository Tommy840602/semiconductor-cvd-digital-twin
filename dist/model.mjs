export const scoped=(devices,line)=>devices.filter(d=>line==='all'||d.line===line);
export const severity=t=>t>95?'嚴重':t>85?'警告':null;
export function evaluate(device,alarms,time){const level=severity(device.samples.at(-1).temperature);if(device.running&&level&&level!==device.active){alarms.unshift({id:`${device.id}-${time}`,deviceId:device.id,time,severity:level,message:`溫度超過 ${level==='嚴重'?95:85}°C 閾值`,acknowledged:false});}device.active=device.running?level:null;}
export function append(device,sample){device.samples.push(sample);device.samples=device.samples.slice(-150);}
export function acknowledge(alarms,id){const a=alarms.find(a=>a.id===id);if(!a)throw Error('找不到告警');a.acknowledged=true;}
export function seed(now=Date.now()){return ['反應爐 01','反應爐 02','冷卻泵 01','空壓機 01','循環泵 02','熱交換器 01'].map((name,i)=>({id:`EQ-00${i+1}`,name,line:i<3?'A':'B',running:i!==4,active:null,samples:Array.from({length:150},(_,j)=>({time:now-(149-j)*2000,temperature:i===4?24:[72,91,43,68,24,57][i]+Math.sin(j/9+i)*2,pressure:i===4?0:3.2+i*.3+Math.sin(j/7)*.12,vibration:i===4?0:1.2+i*.2+Math.sin(j/5)*.2}))}));}
