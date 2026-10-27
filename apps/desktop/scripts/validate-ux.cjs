/* Run against the separate native UX diagnostic build, never a shipping debug port.
   Set PLAYWRIGHT_MODULE to an installed Playwright module. No dependency is added to the app. */
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs = require('node:fs');
const path = require('node:path');
const output = path.resolve(__dirname, '../../../docs/desktop');
const shots = path.join(output, 'screenshots/ux-polish');
async function run() {
 const browser = await chromium.connectOverCDP('http://127.0.0.1:9333');
 const page = browser.contexts()[0].pages().find(p => p.url().includes('wails.localhost'));
 if (!page) throw Error('Native Wails page not found');
 await page.keyboard.press('Control+2');
 await page.getByRole('heading',{name:'Observed processes',exact:true}).waitFor({timeout:65000});
 const cdp = await page.context().newCDPSession(page);
 const {windowId} = await cdp.send('Browser.getWindowForTarget');
 const ratio = await page.evaluate(() => devicePixelRatio);
 const screens=['overview','processes','storage','developer','settings'];
 const rows=[];
 const inspect = () => page.evaluate(() => {
  const el=document.querySelector('.content'), shell=document.querySelector('.app-shell');
  return {viewport:[innerWidth,innerHeight],dpr:devicePixelRatio,height:el.clientHeight,scrollHeight:el.scrollHeight,top:el.scrollTop,
   range:el.scrollHeight-el.clientHeight,pageOverflow:document.documentElement.scrollWidth-innerWidth,
   cardOverflow:[...document.querySelectorAll('.card,.review-card')].filter(e=>e.scrollWidth>e.clientWidth+2).map(e=>e.className),shellHeight:shell.clientHeight,focus:document.activeElement?.id};
 });
 fs.mkdirSync(shots,{recursive:true});
 for (const [width,height] of [[1280,720],[1366,768],[1600,900],[1920,1080]]) {
  // CDP bounds are CSS pixels in WebView2; use host DPI to target physical client dimensions.
  await cdp.send('Browser.setWindowBounds',{windowId,bounds:{width:Math.round(width/ratio),height:Math.round(height/ratio)}});
  await page.waitForTimeout(250);
  for (const theme of ['dark','light']) {
   await page.keyboard.press(`Control+Shift+${theme==='dark'?'D':'L'}`);
   for (let index=0;index<screens.length;index++) {
    const screen=screens[index];
    await page.keyboard.press(`Control+${index+1}`);
    await page.waitForTimeout(100);
    const before=await inspect();
    const filename=`${screen}-${theme}-${width}x${height}`;
    await page.screenshot({path:path.join(shots,`${filename}.png`)});
    const rect=await page.locator('.content').boundingBox();
    await page.mouse.move(rect.x+8,rect.y+rect.height/2);
    await page.mouse.wheel(0,10000);
    await page.waitForTimeout(160);
    const wheel=await inspect();
    await page.locator('.content').focus();
    await page.keyboard.press('Control+Home');
    await page.waitForFunction(() => document.querySelector('.content').scrollTop <= 1);
    await page.keyboard.press('PageDown');
    if(before.range > 1) await page.waitForFunction(() => document.querySelector('.content').scrollTop > 0, null, {timeout:2000});
    await page.waitForTimeout(200);
    const keyboard=await inspect();
    await page.keyboard.press('Control+End');
    await page.waitForTimeout(180);
    const bottom=await inspect();
    await page.screenshot({path:path.join(shots,`${filename}-bottom.png`)});
    const result={screen,theme,physicalTarget:[width,height],before,wheel,keyboard,bottom,
      pass:before.cardOverflow.length===0 && bottom.cardOverflow.length===0 && before.pageOverflow<=1 && before.shellHeight<=before.viewport[1]+1 &&
       (before.range<=1 || wheel.top>0) && (before.range<=1 || keyboard.top>0) && Math.abs(bottom.top-bottom.range)<=2};
    rows.push(result);
    if(!result.pass) console.log('FAIL',JSON.stringify(result));
   }
  }
 }
 fs.writeFileSync(path.join(output,'ux-validation.json'),JSON.stringify({native:true,hostDpiRatio:ratio,rows},null,2));
 console.log(JSON.stringify({cases:rows.length,passed:rows.filter(r=>r.pass).length,failed:rows.filter(r=>!r.pass).length}));
 await browser.close();
}
run().catch(error=>{console.error(error);process.exitCode=1});
