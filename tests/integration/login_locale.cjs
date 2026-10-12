const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch({channel:'chrome',headless:true});try{
for(const choice of ['en','ja',null]){
 const context=await browser.newContext();const page=await context.newPage();let signedIn=false,saved='zh-TW',patches=0;
 await page.addInitScript(()=>localStorage.setItem('cvd.locale','en'));
 await page.route('**/api/**',async route=>{const req=route.request(),path=new URL(req.url()).pathname;let data=[];
 if(path==='/api/auth/csrf')data={token:'fixture'};
 else if(path==='/api/auth/me'){if(!signedIn)return route.fulfill({status:401,json:{message:'Unauthorized'}});data={username:'fixture',role:'Operator'};}
 else if(path==='/api/auth/login'){signedIn=true;data={csrf:'fixture',user:{username:'fixture',role:'Operator'}};}
 else if(path==='/api/auth/preferences'){if(req.method()==='PATCH'){saved=req.postDataJSON().locale;patches++;}data={locale:saved};}
 else if(path.includes('/statistics'))data={};
 await route.fulfill({json:data});});
 await page.goto(process.env.CVD_UI_TEST_URL||'http://localhost:18000');
 await page.locator('input[autocomplete=username]:enabled').waitFor();
 if(choice)await page.locator('.language-switch select').selectOption(choice);
 await page.locator('input[autocomplete=username]').fill('fixture');await page.locator('input[type=password]').fill('fixture-password');await page.locator('.auth-card button.primary').click();
 await page.locator('.shell').waitFor();assert.equal(await page.locator('.language-switch select').inputValue(),choice||'zh-TW');assert.equal(saved,choice||'zh-TW');assert.equal(patches,choice?1:0);
 await page.reload();await page.locator('.shell').waitFor();assert.equal(await page.locator('.language-switch select').inputValue(),choice||'zh-TW');assert.equal(patches,choice?1:0);
 await context.close();console.log('PASS login choice',choice,'and reload');
}
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
