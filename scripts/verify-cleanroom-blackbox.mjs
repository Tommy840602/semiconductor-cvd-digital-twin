import {spawn} from 'node:child_process';
import fs from 'node:fs/promises';
const child=spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',['--headless','--no-first-run','--no-default-browser-check','--remote-debugging-port=9238','--user-data-dir=/tmp/cvd-disposition-ui-'+Date.now(),'about:blank'],{stdio:'ignore'});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));let ws;
try {
 let tabs;for(let n=0;n<60;n++){try{tabs=await(await fetch('http://127.0.0.1:9238/json')).json();if(tabs.length)break;}catch{}await sleep(300);}
 ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map(),errors=[];ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails);if(pending.has(m.id)){const [r,j]=pending.get(m.id);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result)}};
 const call=(method,params={})=>new Promise((r,j)=>{const id=++seq;pending.set(id,[r,j]);ws.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.exception?.description);return r.result.value;};
 const wait=async expression=>{for(let n=0;n<90;n++){if(await evaluate(expression))return;await sleep(300);}throw Error('Timeout '+expression+' '+await evaluate('document.body.innerText'));};
 const click=text=>evaluate(`(()=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)});if(!b)throw Error('missing button');b.click()})()`);
 await call('Runtime.enable');await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await call('Page.navigate',{url:'http://127.0.0.1:22000/'});
 await wait('!!document.querySelector("input[autocomplete=username]")&&!document.querySelector(".auth-card button").disabled');
 await evaluate(`(()=>{const f=document.querySelectorAll('.auth-card input');[${JSON.stringify(process.env.CVD_UI_USER||'disposition.ee')},'Disposition-Test-2026!'].forEach((v,i)=>{f[i].value=v;f[i].dispatchEvent(new Event('input',{bubbles:true}))});document.querySelector('.auth-card').requestSubmit()})()`);
 await wait('!!document.querySelector(".cr-detail")');
 const checks={selected:[],errors};await click('手動操控');await wait('!!document.querySelector(".cr-stick")');
 await call('Input.dispatchKeyEvent',{type:'keyDown',key:'w',code:'KeyW'});await sleep(500);await call('Input.dispatchKeyEvent',{type:'keyUp',key:'w',code:'KeyW'});
 await click('全場俯視');await wait('document.querySelector(".cr-caption").innerText.includes("俯視中")');
 await click('自動巡走');await wait('document.querySelector(".cr-caption").innerText.includes("自動巡走")');
 for(let n=1;n<=10;n++){
  await click('設備清單');await evaluate(`document.querySelectorAll('.cr-list button')[${n-1}].click()`);
  await wait(`!document.querySelector('.cleanroom') && document.querySelector('.nav-equipment').textContent.includes('08A-${String(n).padStart(2,'0')}')`);
  checks.selected.push(n);await click('全廠總覽');await wait('!!document.querySelector(".cr-detail")');
 }
 await call('Emulation.setDeviceMetricsOverride',{width:390,height:844,deviceScaleFactor:1,mobile:true});await sleep(2000);
 checks.mobile=await evaluate('({width:document.documentElement.scrollWidth,viewport:innerWidth,caption:document.querySelector(".cr-caption").innerText})');
 await click('手動操控');await wait('!!document.querySelector(".cr-stick")');
 await evaluate(`document.querySelector('.cr-canvas canvas').dispatchEvent(new Event('webglcontextlost',{cancelable:true}))`);await wait('document.querySelectorAll(".cr-list button").length===10');checks.fallback=true;
 await click('重新載入 3D');await wait('!!document.querySelector(".cr-detail")');checks.retry=true;
 if(checks.mobile.width>391||errors.length)throw Error(JSON.stringify(checks));await fs.writeFile('.runtime/disposition-3d-proof.json',JSON.stringify(checks,null,2));console.log(JSON.stringify(checks));

}finally{ws?.close();child.kill('SIGTERM');}
