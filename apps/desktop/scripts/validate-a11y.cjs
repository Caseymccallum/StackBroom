const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const fs=require('node:fs'); const path=require('node:path');
(async()=>{
 const browser=await chromium.connectOverCDP('http://127.0.0.1:9333');const page=browser.contexts()[0].pages().find(p=>p.url().includes('wails.localhost'));
 await page.addScriptTag({path:process.env.AXE_CORE_SCRIPT||require.resolve('axe-core/axe.min.js')});
 const rows=[];
 for(const theme of ['dark','light']){
  await page.keyboard.press(`Control+Shift+${theme==='dark'?'D':'L'}`);
  for(let index=0;index<5;index++){
   await page.keyboard.press(`Control+${index+1}`);await page.waitForTimeout(100);
   const violations=await page.evaluate(async()=>{
    const result=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}});
    return result.violations.map(v=>({id:v.id,impact:v.impact,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))}));
   });
   rows.push({theme,screen:index+1,violations});
  }
 }
 fs.writeFileSync(path.resolve(__dirname,'../../../docs/desktop/ux-accessibility.json'),JSON.stringify(rows,null,2));
 console.log(JSON.stringify({cases:rows.length,violations:rows.reduce((n,row)=>n+row.violations.length,0)}));await browser.close();
})().catch(error=>{console.error(error);process.exitCode=1});
