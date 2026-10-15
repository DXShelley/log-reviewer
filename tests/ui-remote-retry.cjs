const {chromium}=require('playwright');
const fs=require('fs'),path=require('path'),assert=require('assert');

async function main(){
 const root=path.resolve(__dirname,'..');
 const url=process.env.LR_TEST_URL||fs.readFileSync(path.join(root,'.work/gui.out'),'utf8').match(/HTTP: (http:\/\/\S+)/)?.[1];
 if(!url)throw Error('HTTP compatibility URL not found; provide LR_TEST_URL for a running server');
 const browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});
 const context=await browser.newContext();
 try{
  const page=await context.newPage(),requests=[],errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/*',route=>{
   const pathname=new URL(route.request().url()).pathname;
   if(pathname.startsWith('/api/'))return route.fallback();
   const file=path.join(root,'internal/server/web',pathname==='/'?'index.html':pathname);
   return fs.existsSync(file)?route.fulfill({path:file}):route.fallback();
  });
  await page.route('**/api/remote-hosts',route=>route.fulfill({json:{hosts:[]}}));
  await page.route('**/api/remote-fetch',route=>{
   if(route.request().method()==='POST'){
    requests.push(route.request().postDataJSON());
    return route.fulfill({json:{id:String(requests.length)}});
   }
   if(route.request().method()==='DELETE')return route.fulfill({json:{accepted:true}});
   return route.fulfill({json:{id:String(requests.length),busy:false,stage:'failed',error:'SFTP subsystem unavailable: permission denied',files:0,total_files:0,bytes:0,total:0}});
  });
  await page.goto(url,{timeout:10000});
  await page.locator('#remote-button').click();
  for(const [field,value] of Object.entries({host:'127.0.0.1',port:'22',user:'reader',path:'/logs',password:'retry-secret'}))await page.locator('#remote-'+field).fill(value);
  async function submit(){await page.locator('#remote-submit').click();try{await page.waitForFunction(()=>!document.getElementById('remote-submit').disabled&&document.getElementById('remote-status').textContent.includes('SFTP subsystem unavailable: permission denied'),null,{timeout:5000})}catch(e){console.error({requests:requests.length,errors,status:await page.locator('#remote-status').textContent(),invalid:await page.locator('#remote-form :invalid').evaluateAll(nodes=>nodes.map(n=>n.id))});throw e}}
  await submit();
  assert.equal(await page.locator('#remote-password').inputValue(),'retry-secret');
  await page.locator('#remote-path').fill('/other-logs');await submit();
  assert.equal(requests.length,2);assert.equal(requests[1].password,'retry-secret');
  await page.locator('#remote-user').fill('other');assert.equal(await page.locator('#remote-password').inputValue(),'');
  await page.locator('#remote-user').fill('reader');assert.equal(await page.locator('#remote-password').inputValue(),'retry-secret');
  await page.locator('#close-remote').click();await page.locator('#remote-button').click();
  assert.equal(await page.locator('#remote-password').inputValue(),'retry-secret');
  await submit();assert.equal(requests[2].password,'retry-secret');
  assert.equal(errors.length,0,errors.join('\n'));
  await page.screenshot({path:path.join(root,'.work/remote-retry.png')});
  console.log('PASS: retained password on failure, path change, reopen; isolated accounts; exact component error displayed');
 }finally{await context.close();await browser.close()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
