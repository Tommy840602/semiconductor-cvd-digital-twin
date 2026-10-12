// Read-only public deployment QA: login, navigation, hover and screenshots; no business commands.
import {spawn,execFile} from 'node:child_process';
import {promisify} from 'node:util';
const exec=promisify(execFile);
import fs from 'node:fs/promises';
const profile='/tmp/cvd-secs-production-ui-'+Date.now();
const child=spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',['--headless','--no-first-run','--no-default-browser-check','--remote-debugging-port=9229','--user-data-dir='+profile,'about:blank'],{stdio:'ignore'});
const origin='https://enhanced-del-stylish-settled.trycloudflare.com/';
const prefix='secs-production';
const deployment=JSON.parse(await fs.readFile('.runtime/secs-production-compose.json','utf8'));
const password=(await fs.readFile(deployment.secrets.leader_password.file,'utf8')).trim();
let ws;const sleep=ms=>new Promise(r=>setTimeout(r,ms));
try{
 let tabs;for(let i=0;i<60;i++){try{tabs=await(await fetch('http://127.0.0.1:9229/json')).json();if(tabs.length)break;}catch{}await sleep(500);}
 ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map();ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.id&&pending.has(m.id)){const {r,j}=pending.get(m.id);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result)}};
 const call=(method,params={})=>new Promise((r,j)=>{const id=++seq;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>(await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true})).result.value;
 async function wait(expression){for(let i=0;i<60;i++){if(await evaluate(expression))return;await sleep(500);}console.log(await evaluate('document.body.innerText'));const failure=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/'+prefix+'-ui-failure.png',Buffer.from(failure.data,'base64'));throw Error('UI timeout: '+expression);}
 await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await call('Page.navigate',{url:origin});
 await wait('!!document.querySelector("input[autocomplete=username]") && !document.querySelector(".auth-card button").disabled');
 await evaluate(`(()=>{const fields=document.querySelectorAll('.auth-card input');for(const [i,v] of ${JSON.stringify(['cvd.leader',password])}.entries()){fields[i].value=v;fields[i].dispatchEvent(new Event('input',{bubbles:true}));}document.querySelector('.auth-card').requestSubmit();})()`);
 await wait('document.querySelectorAll(".fleet-card").length===10');
 if(!(await evaluate('document.body.innerText.includes("SECS/GEM")')))throw Error('old protocol label');
 let screenshot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/'+prefix+'-desktop.png',Buffer.from(screenshot.data,'base64'));
 await evaluate(`(()=>{const s=document.querySelector('.fleet-toolbar select');s.value='TEL-CVDTIN-08A-02';s.dispatchEvent(new Event('change',{bubbles:true}));})()`);
 await wait('!!document.querySelector(".recipe-editor") && document.querySelector(".nav-equipment").textContent.includes("08A-02")');
 await evaluate(`document.querySelector('.recipe-editor summary').click()`);await wait('!!document.querySelector(".recipe-editor form")');
 await wait(`!!document.querySelector('g.equipment-target[aria-label="PM2 製程詳情"]')`);
 const pm=await evaluate(`(()=>{const e=document.querySelector('g.equipment-target[aria-label="PM2 製程詳情"]');e.scrollIntoView({block:'center'});const r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()`);
 await call('Input.dispatchMouseEvent',{type:'mouseMoved',...pm});
 await wait(`Array.from(document.querySelectorAll('h2')).some(h=>h.textContent.includes('PM2 製程詳情'))`);
 let text=await evaluate('document.querySelector(".recipe-editor").innerText');if(!text.includes('複製至 TEL-CVDTIN-08A-02'))throw Error('copy destination mismatch');
 await evaluate(`(()=>{const s=document.querySelector('.fleet-toolbar select');s.value='TEL-CVDTIN-08A-03';s.dispatchEvent(new Event('change',{bubbles:true}));s.value='TEL-CVDTIN-08A-10';s.dispatchEvent(new Event('change',{bubbles:true}));})()`);
 await wait('document.querySelector(".nav-equipment").textContent.includes("08A-10")');
 await evaluate(`document.querySelector('.fleet-toolbar button').click()`);await call('Emulation.setDeviceMetricsOverride',{width:390,height:844,deviceScaleFactor:1,mobile:true});
 await sleep(500);await evaluate('window.scrollTo(0,0)');let width=await evaluate('({page:document.documentElement.scrollWidth,viewport:innerWidth,cards:document.querySelectorAll(".fleet-card").length})');
 if(width.page>width.viewport)throw Error('mobile overflow');
 screenshot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/'+prefix+'-mobile.png',Buffer.from(screenshot.data,'base64'));
 await fs.writeFile('.runtime/'+prefix+'-ui.json',JSON.stringify({pmHover:true,desktopTenCards:true,selectionAndRecipeDestination:true,rapidSwitchTo10:true,mobile:width},null,2));console.log(JSON.stringify(width));
}finally{ws?.close();child.kill('SIGTERM');await new Promise(r=>{if(child.exitCode!==null)r();else child.once('exit',r)});await fs.rm(profile,{recursive:true,force:true});}
