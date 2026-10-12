import {spawn} from 'node:child_process';
import fs from 'node:fs/promises';
const child=spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',['--headless','--no-first-run','--no-default-browser-check','--remote-debugging-port=9232','--user-data-dir=/tmp/cvd-cleanroom-ui-'+Date.now(),'about:blank'],{stdio:'ignore'});
let ws;const sleep=ms=>new Promise(r=>setTimeout(r,ms));
try{
 let tabs;for(let i=0;i<60;i++){try{tabs=await(await fetch('http://127.0.0.1:9232/json')).json();if(tabs.length)break;}catch{}await sleep(500);}
 ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map();ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.id&&pending.has(m.id)){const {r,j}=pending.get(m.id);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result)}};
 const call=(method,params={})=>new Promise((r,j)=>{const id=++seq;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.text+': '+r.exceptionDetails.exception?.description);return r.result.value;};
 async function wait(expression){for(let i=0;i<60;i++){if(await evaluate(expression))return;await sleep(500);}console.log(await evaluate('document.body.innerText'));const failure=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/fleet-ui-failure.png',Buffer.from(failure.data,'base64'));throw Error('UI timeout: '+expression);}
 await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await call('Page.navigate',{url:(process.env.CVD_UI_URL||'http://localhost:18001/')});
 await wait('!!document.querySelector("input[autocomplete=username]") && !document.querySelector(".auth-card button").disabled');
 await evaluate(`(()=>{const fields=document.querySelectorAll('.auth-card input');for(const [i,v] of ['cvd.leader',${JSON.stringify((await fs.readFile(process.env.CVD_UI_PASSWORD_FILE||'.runtime/secrets/leader-password','utf8')).trim())}].entries()){fields[i].value=v;fields[i].dispatchEvent(new Event('input',{bubbles:true}));}document.querySelector('.auth-card').requestSubmit();})()`);
 await wait('!!document.querySelector(".cr-detail")');
 const component='document.querySelector(".cleanroom").__vueParentComponent';
 const diag=()=>evaluate(component+'.exposed.diagnostics()');
 const click=label=>evaluate(`Array.from(document.querySelectorAll('.cleanroom button')).find(b=>b.textContent.trim()===${JSON.stringify(label)}).click()`);
 const checks={};
 const originalFleet=await evaluate(`JSON.stringify(${component}.props.fleet)`);
 await evaluate(`(()=>{const f=${component}.props.fleet;f[1].faults=1;f[1].sourceTime=new Date().toISOString();f[1].communicationQuality='good';f[2].communicationQuality='stale';})()`);await sleep(80);
 checks.isolatedStatuses=(await diag()).statuses;if(checks.isolatedStatuses[1].label!=='故障'||checks.isolatedStatuses[2].label!=='資料過期')throw Error('status isolation failed');
 await evaluate(`(()=>{const original=JSON.parse(${JSON.stringify(originalFleet)});${component}.props.fleet.forEach((m,i)=>Object.assign(m,original[i]));})()`);

 const requests=[];await call('Network.enable');const oldHandler=ws.onmessage;ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.method==='Network.requestWillBeSent'&&m.params.request.method==='POST')requests.push(m.params.request.url);oldHandler(e);};
 await evaluate(`${component}.props.state.x=-2;${component}.props.state.z=-15.6;${component}.props.state.waypoint=0;${component}.props.state.pause=0`);
 const visited=new Set();for(let i=0;i<(process.env.CVD_QUICK?0:56);i++){visited.add((await diag()).waypoint);await sleep(1000);}checks.patrolVisited=[...visited];if(!process.env.CVD_QUICK&&visited.size!==10)throw Error('patrol incomplete');

 await click('手動操控');await sleep(300);checks.initial=await diag();
 await call('Input.dispatchKeyEvent',{type:'keyDown',key:'w',code:'KeyW'});await sleep(1000);await call('Input.dispatchKeyEvent',{type:'keyUp',key:'w',code:'KeyW'});checks.moved=await diag();
 if(checks.moved.position.z>=checks.initial.position.z-.5)throw Error('W movement failed');
 await click('全場俯視');let before=await diag();await call('Input.dispatchKeyEvent',{type:'keyDown',key:'w',code:'KeyW'});await sleep(500);await call('Input.dispatchKeyEvent',{type:'keyUp',key:'w',code:'KeyW'});checks.overview=await diag();if(checks.overview.position.z!==before.position.z)throw Error('overview moved');
 await click('自動巡走');await sleep(500);checks.auto=await diag();await click('手動操控');checks.toggle=await diag();if(Math.hypot(checks.toggle.position.x-checks.auto.position.x,checks.toggle.position.z-checks.auto.position.z)>.3)throw Error('mode teleported');
 await click('全場俯視');await sleep(2500);checks.desktop=await diag();
 await evaluate('document.querySelector(".cr-stage").scrollIntoView({block:"center"})');
 const target=checks.desktop.targets.find(t=>t.id.endsWith('-02'));const rect=await evaluate('(()=>{const r=document.querySelector(".cr-canvas").getBoundingClientRect();return {x:r.x,y:r.y}})()');
 await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:rect.x+target.x,y:rect.y+target.y});await sleep(200);checks.meshHover=await evaluate('document.querySelector(".cr-detail h3").textContent');if(!checks.meshHover.endsWith('-02'))throw Error('3D hover identity mismatch');
 let shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/cleanroom-desktop.png',Buffer.from(shot.data,'base64'));
 checks.selection=[];
 for(let n=1;n<=10;n++){
  await click('設備清單');
  await evaluate(`document.querySelectorAll('.cr-list button')[${n-1}].click()`);
  await wait(`!document.querySelector('.cleanroom') && document.querySelector('.nav-equipment').textContent.includes('08A-${String(n).padStart(2,'0')}')`);
  checks.selection.push(n);await evaluate('document.querySelector(".fleet-toolbar button").click()');await wait('!!document.querySelector(".cr-detail")');
 }
 checks.returned=await diag();if(checks.returned.position.z!==checks.desktop.position.z)throw Error('return reset');
 await call('Emulation.setDeviceMetricsOverride',{width:390,height:844,deviceScaleFactor:1,mobile:true});await sleep(2500);
 checks.mobile=await diag();
 await click('第三人稱跟隨');await evaluate('document.querySelector(".cr-stage").scrollIntoView({block:"center"})');await sleep(300);
 const stickBox=await evaluate('(()=>{const r=document.querySelector(".cr-stick").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()');
 const touchBefore=await diag();await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:stickBox.x,y:stickBox.y-30}]});await sleep(500);await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});checks.touchAfter=await diag();if(checks.touchAfter.position.z>=touchBefore.position.z)throw Error('touch failed');
 await click('全場俯視');
checks.mobileSize=await evaluate('({width:document.documentElement.scrollWidth,viewport:innerWidth,fps:document.querySelector(".cr-caption").innerText})');
 await evaluate('document.querySelector(".cr-stage").scrollIntoView({block:"center"})');shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/cleanroom-mobile.png',Buffer.from(shot.data,'base64'));
 await click('第三人稱跟隨');await sleep(500);shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/cleanroom-follow.png',Buffer.from(shot.data,'base64'));
 // Context loss must expose working fallback, then retry must recover.
 await evaluate(`document.querySelector('.cr-canvas canvas').dispatchEvent(new Event('webglcontextlost',{cancelable:true}))`);
 await wait('document.querySelectorAll(".cr-list button").length===10');checks.contextFallback=true;
 await click('重新載入 3D');await wait('!!document.querySelector(".cr-detail")');checks.retry=true;
 // Fail the GLB request on a fresh component mount.
 await call('Network.enable');await call('Network.setBlockedURLs',{urls:['*tel-cvd.glb*']});
 await click('重新載入 3D').catch(()=>{});
 await evaluate('document.querySelector(".cr-detail button").click()');await wait('!document.querySelector(".cleanroom")');await evaluate('document.querySelector(".fleet-toolbar button").click()');await wait('!!document.querySelector(".cr-loading button")');checks.assetFallback=await evaluate('document.querySelectorAll(".cr-list button").length===10');
 await call('Network.setBlockedURLs',{urls:[]});await click('重新載入 3D');await wait('!!document.querySelector(".cr-detail")');
 checks.unexpectedPosts=requests.filter(u=>!u.includes('/auth/'));if(checks.unexpectedPosts.length)throw Error('unexpected mutation');
 await fs.writeFile(process.env.CVD_QUICK?'.runtime/cleanroom-quick-proof.json':'.runtime/cleanroom-proof.json',JSON.stringify(checks,null,2));console.log(JSON.stringify({patrolVisited:checks.patrolVisited,selection:checks.selection,mobile:checks.mobileSize,contextFallback:checks.contextFallback,assetFallback:checks.assetFallback,unexpectedPosts:checks.unexpectedPosts}));

}finally{ws?.close();child.kill('SIGTERM');}
