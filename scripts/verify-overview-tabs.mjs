import {spawn} from 'node:child_process';
import fs from 'node:fs/promises';
const child=spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',['--headless','--no-first-run','--no-default-browser-check','--remote-debugging-port=9244','--user-data-dir=/tmp/cvd-disposition-ui-'+Date.now(),'about:blank'],{stdio:'ignore'});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));let ws;
try {
 let tabs;for(let n=0;n<60;n++){try{tabs=await(await fetch('http://127.0.0.1:9244/json')).json();if(tabs.length)break;}catch{}await sleep(300);}
 ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map(),errors=[];ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails);if(pending.has(m.id)){const [r,j]=pending.get(m.id);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result)}};
 const call=(method,params={})=>new Promise((r,j)=>{const id=++seq;pending.set(id,[r,j]);ws.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.exception?.description);return r.result.value;};
 const wait=async expression=>{for(let n=0;n<200;n++){if(await evaluate(expression))return;await sleep(300);}throw Error(JSON.stringify(errors)+' Timeout '+expression+' '+await evaluate('document.body.innerText'));};
 const click=text=>evaluate(`(()=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)});if(!b)throw Error('missing button');b.click()})()`);
 await call('Runtime.enable');await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await call('Page.navigate',{url:process.env.CVD_UI_URL||'http://127.0.0.1:22000/'});
 await wait('!!document.querySelector("input[autocomplete=username]")&&!document.querySelector(".auth-card button").disabled');
 await evaluate(`(()=>{const f=document.querySelectorAll('.auth-card input');[${JSON.stringify(process.env.CVD_UI_USER||'disposition.ee')},${JSON.stringify(process.env.CVD_UI_PASSWORD_FILE?(await fs.readFile(process.env.CVD_UI_PASSWORD_FILE,'utf8')).trim():'Disposition-Test-2026!')}].forEach((v,i)=>{f[i].value=v;f[i].dispatchEvent(new Event('input',{bubbles:true}))});document.querySelector('.auth-card').requestSubmit()})()`);

 await wait('document.querySelectorAll(".overview-tabs [role=tab]").length===5');
 const visible=selector=>evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});return !!el&&el.getBoundingClientRect().height>0&&getComputedStyle(el).display!=='none'})()`);
 if(!await visible('.cleanroom')||await visible('.util-panel'))throw Error('Default should show only 3D');
 for(const [tab,selector] of [['utilization','.util-summary'],['timeline','.stats-gantt'],['distribution','.stats-state'],['production','.stats-production']]){
  await evaluate(`document.getElementById('overview-tab-${tab}').click()`);
  await wait('document.querySelectorAll(".util-tools button").length===10');
  if(!await visible(selector)||await visible('.cleanroom'))throw Error('Wrong tab '+tab);
  const count=await evaluate(`[...document.querySelectorAll('.util-summary,.stats-gantt,.stats-state,.stats-production')].filter(e=>e.getBoundingClientRect().height>0).length`);
  if(count!==1)throw Error('More than one statistics section visible');
 }
 await evaluate(`(()=>{const t=document.getElementById('overview-tab-distribution');t.click();const s=document.querySelector('.stats-filter select');s.value='TEL-CVDTIN-08A-02';s.dispatchEvent(new Event('change',{bubbles:true}));document.getElementById('overview-tab-timeline').click();document.getElementById('overview-tab-distribution').click()})()`);
 if(await evaluate('document.querySelector(".stats-filter select").value')!=='TEL-CVDTIN-08A-02')throw Error('Lost selection');
 await call('Emulation.setDeviceMetricsOverride',{width:390,height:844,deviceScaleFactor:1,mobile:true});
 await evaluate('document.querySelector(".overview-tabs").scrollIntoView()');await sleep(500);
 const mobile=await evaluate('({width:document.documentElement.scrollWidth,viewport:innerWidth})');if(mobile.width>mobile.viewport)throw Error('Overflow');
 let shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/tabs-mobile.png',Buffer.from(shot.data,'base64'));
 await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});
 await evaluate('document.getElementById("overview-tab-scene").click()');await wait('!!document.querySelector(".cr-enter")');
 await evaluate('document.activeElement?.blur()');await call('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});await call('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});await wait('!document.querySelector(".cleanroom")');
 await call('Input.dispatchKeyEvent',{type:'keyDown',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});await call('Input.dispatchKeyEvent',{type:'keyUp',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});await wait('!!document.querySelector(".overview-tabs")');
 if(errors.length)throw Error(JSON.stringify(errors));console.log(JSON.stringify({tabs:5,oneSectionAtATime:true,selectionPreserved:true,mobile,enterEscape:true,runtimeErrors:0}));
}finally{ws?.close();child.kill('SIGTERM');}
