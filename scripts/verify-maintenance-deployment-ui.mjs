import {spawn} from 'node:child_process';
import fs from 'node:fs/promises';
const child=spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',['--headless','--no-first-run','--no-default-browser-check','--remote-debugging-port=9238','--user-data-dir=/tmp/cvd-disposition-ui-'+Date.now(),'about:blank'],{stdio:'ignore'});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));let ws;
try {
 let tabs;for(let n=0;n<60;n++){try{tabs=await(await fetch('http://127.0.0.1:9238/json')).json();if(tabs.length)break;}catch{}await sleep(300);}
 ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map(),errors=[];ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails);if(pending.has(m.id)){const [r,j]=pending.get(m.id);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result)}};
 const call=(method,params={})=>new Promise((r,j)=>{const id=++seq;pending.set(id,[r,j]);ws.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.exception?.description);return r.result.value;};
 const wait=async expression=>{for(let n=0;n<200;n++){if(await evaluate(expression))return;await sleep(300);}throw Error('Timeout '+expression+' '+await evaluate('document.body.innerText'));};
 const click=text=>evaluate(`(()=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)});if(!b)throw Error('missing button');b.click()})()`);
 await call('Runtime.enable');await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await call('Page.navigate',{url:process.env.CVD_UI_URL||'http://127.0.0.1:22000/'});
 await wait('!!document.querySelector("input[autocomplete=username]")&&!document.querySelector(".auth-card button").disabled');
 await evaluate(`(()=>{const f=document.querySelectorAll('.auth-card input');[${JSON.stringify(process.env.CVD_UI_USER||'disposition.ee')},${JSON.stringify(process.env.CVD_UI_PASSWORD_FILE?(await fs.readFile(process.env.CVD_UI_PASSWORD_FILE,'utf8')).trim():'Disposition-Test-2026!')}].forEach((v,i)=>{f[i].value=v;f[i].dispatchEvent(new Event('input',{bubbles:true}))});document.querySelector('.auth-card').requestSubmit()})()`);
 await wait('document.querySelectorAll(".util-tools button").length===10');
 await evaluate('document.querySelectorAll(".util-tools button")[7].click()');
 await wait('!!document.querySelector(".maintenance-panel")');
 await wait('[...document.querySelectorAll(".maintenance-actions button")].some(b=>b.textContent.trim()==="申請保養"&&!b.disabled)');
 const proof=await evaluate('({url:location.href,equipment:document.querySelector(".nav-equipment b").textContent,text:document.querySelector(".maintenance-panel").innerText,versionAssets:[...document.scripts].map(s=>s.src)})');
 if(errors.length)throw Error(JSON.stringify(errors));
 await evaluate('document.querySelector(".maintenance-panel").scrollIntoView()');
 const shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/maintenance-production.png',Buffer.from(shot.data,'base64'));
 await fs.writeFile('.runtime/maintenance-production-ui.json',JSON.stringify(proof,null,2));console.log(JSON.stringify(proof));
}finally{ws?.close();child.kill('SIGTERM');}
