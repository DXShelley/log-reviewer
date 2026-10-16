const {chromium}=require('playwright');
const {spawn}=require('child_process');
const fs=require('fs'),os=require('os'),path=require('path'),assert=require('assert');
async function main(){
 const root=path.resolve(__dirname,'..'),dir=fs.mkdtempSync(path.join(os.tmpdir(),'lr-settings-regression-'));
 const binary=process.env.LR_TEST_BINARY||path.join(root,'dist/log-reviewer-windows-amd64.exe');
 let child,browser,context;
 async function stop(){if(child){const current=child;child=null;await new Promise(resolve=>{if(current.exitCode!==null)return resolve();current.once('exit',resolve);current.kill()})}}
 async function start(flags=[]){
  child=spawn(binary,['gui','--host','127.0.0.1','--no-browser','--port','0','--https-port','0','--state-file',path.join(dir,'state.json'),'--tls-file',path.join(dir,'tls.pem'),'--rule-dir',path.join(dir,'rules'),...flags],{windowsHide:true});
  return new Promise((resolve,reject)=>{let output='';const timer=setTimeout(()=>reject(Error('startup timeout')),10000);child.on('error',reject);child.stdout.on('data',b=>{output+=b;const match=output.match(/HTTP: (http:\/\/\S+)/);if(match){clearTimeout(timer);resolve(match[1])}})});
 }
 try{
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});context=await browser.newContext();const page=await context.newPage(),errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  for(const [flags,fileVisible,pathVisible]of [[[],false,true],[['--show-file-picker'],true,false],[['--show-file-picker','--show-path-input'],true,false],[['--show-path-input=false'],true,false]]){
   const url=await start(flags);await page.goto(url,{timeout:10000});await page.waitForFunction(()=>fileSettings!==null);
   assert(await page.locator('#upload-button').isVisible());assert.equal(await page.locator('#file-picker-button').isVisible(),fileVisible);assert.equal(await page.locator('#path-toggle').isVisible(),pathVisible);
   if(!flags.length){
    await page.locator('#path-toggle').click();assert(await page.locator('#paths-panel').isVisible());
    const log=path.join(dir,'app.log');fs.writeFileSync(log,'2026-10-15 12:00:00 ERROR service : request failed\n');
    await page.locator('#paths').fill(log);await page.locator('#add-paths').click();await page.waitForFunction(()=>files.length===1);
    await page.locator('#analyze').click();await page.waitForFunction(()=>!busy&&groups.length===1);
    for(const [width,height]of [[1920,920],[1440,900],[768,650],[390,844],[390,480]]){
     await page.setViewportSize({width,height});await page.locator('#access-open').click();
     for(const tab of [0,1]){
      await page.locator('#settings-tab-'+tab).click();assert.equal(await page.locator('#access-dialog button:visible').filter({hasText:/^保存$/}).count(),1);
      const save=page.locator(tab?'#file-settings-save':'#access-save');await save.scrollIntoViewIfNeeded();
      const bounds=await save.boundingBox();assert(bounds.x>=0&&bounds.y>=0&&bounds.x+bounds.width<=width&&bounds.y+bounds.height<=height,JSON.stringify(bounds));
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
      await page.screenshot({path:path.join(dir,`settings-${width}-${height}-${tab}.png`)});
     }
     await page.locator('#access-close').click();
    }
    await page.locator('#access-open').click();await page.locator('#settings-tab-0').focus();await page.keyboard.press('ArrowRight');assert.equal(await page.locator('#settings-tab-1').getAttribute('aria-selected'),'true');
    await page.locator('#file-patterns').fill('*.log\n*.txt');await page.locator('#all').check();await page.locator('#file-settings-save').click();await page.locator('#access-dialog').waitFor({state:'hidden'});
    const saved=JSON.parse(fs.readFileSync(path.join(dir,'state.json')));assert.deepEqual(saved.file_patterns,['*.log','*.txt']);assert.equal(saved.include_all,true);
    await page.reload();await page.waitForFunction(()=>fileSettings!==null);assert(await page.locator('#all').isChecked());
    await page.locator('#access-open').click();await page.locator('#settings-tab-1').click();
    await page.locator('#file-patterns').fill('[');await page.locator('#file-settings-save').click();await page.waitForFunction(()=>document.getElementById('file-settings-status').textContent.includes('有效'));
    assert.deepEqual(JSON.parse(fs.readFileSync(path.join(dir,'state.json'))).file_patterns,['*.log','*.txt']);
    await page.getByRole('button',{name:'恢复默认值',exact:true}).click();await page.waitForFunction(()=>document.getElementById('file-patterns').value.includes('*.trace'));
    let release,seen;const held=new Promise(resolve=>release=resolve),started=new Promise(resolve=>seen=resolve);
    await page.route('**/api/import-settings',async route=>{if(route.request().method()!=='PUT')return route.continue();seen();await held;await route.continue()});
    await page.locator('#file-settings-save').click();await started;
    assert(await page.locator('#access-close').isDisabled());assert(await page.locator('#settings-tab-0').isDisabled());await page.keyboard.press('Escape');assert(await page.locator('#access-dialog').isVisible());
    release();await page.locator('#access-dialog').waitFor({state:'hidden'});await page.unroute('**/api/import-settings');
   }else if(fileVisible){await page.locator('#upload-button').click();assert(await page.locator('#import-files').isVisible());assert(await page.locator('#import-directory').isVisible());await page.locator('#close-import').click()}
   await stop();
  }
  assert.deepEqual(errors,[]);console.log('PASS settings regression: default path import and analysis, explicit flags, one save per tab, keyboard navigation, persistence, five viewports; evidence: '+dir);
 }finally{if(context)await context.close();if(browser)await browser.close();await stop()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
