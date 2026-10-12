const rows={
 chamber:['製程腔體','Process chamber','プロセスチャンバー'],details:['查看詳細','View details','詳細を表示'],
 title:['OpenCV 視覺檢查','OpenCV inspection','OpenCV 外観検査'],
 sourceNotice:['公開資料重播／模擬影像；不是此晶圓的實測影像','Dataset replay / synthetic images; not a measurement of this wafer','公開データ再生／合成画像。このウェーハの実測ではありません'],
 map:['缺陷分布','Defect pattern','欠陥分布'],surface:['表面影像','Surface image','表面画像'],original:['原圖','Original','原画像'],overlay:['缺陷標記','Overlay','欠陥表示'],
 empty:['尚未執行視覺檢查','Not inspected yet','未検査'],error:['無法取得檢查資料，請重試','Unable to load inspection. Retry.','検査データを取得できません。再試行してください'],
 retry:['重新載入','Reload','再読み込み'],reanalyze:['重新分析原圖','Reanalyze original','原画像を再解析'],ack:['確認告警','Acknowledge','警報確認'],
 revision:['檢查版本','Inspection revision','検査バージョン'],source:['影像來源','Image source','画像ソース'],dataset:['公開資料重播','Dataset replay','公開データ再生'],synthetic:['模擬影像','Synthetic image','合成画像'],
 queued:['排隊中','Queued','待機中'],running:['檢查中','Inspecting','検査中'],completed:['已完成','Completed','完了'],failed:['檢查失敗','Inspection failed','検査失敗'],
 pass:['未檢出缺陷','No defect detected','欠陥未検出'],review:['需檢視異常','Review defects','異常要確認'],uncertain:['無法確定','Uncertain','判定不確定'],
 expired:['影像與結果已過保存期限','Images and results expired','画像・結果の保存期限切れ'],partial:['部分檢查失敗；不代表合格','Partial failure; not a passing result','一部検査失敗。合格ではありません'],
 pixel:['尺寸以像素顯示；無實際比例尺','Pixel dimensions; no physical calibration','ピクセル単位。実寸校正なし'],
 defects:['缺陷列表','Defects','欠陥一覧'],area:['面積','Area','面積'],size:['尺寸','Size','サイズ'],position:['位置','Position','位置'],none:['無','None','なし'],
 center:['中心集中','Center','中心'],donut:['環狀','Donut','リング'], 'edge-ring':['邊緣環狀','Edge ring','外周リング'],'edge-loc':['邊緣局部','Edge local','外周局所'],loc:['局部集中','Local','局所'],random:['隨機分布','Random','ランダム'],'near-full':['近全片','Near full','ほぼ全面'],scratch:['刮傷候選','Scratch candidate','スクラッチ候補'],particle:['顆粒候選','Particle candidate','粒子候補'],spot:['斑點候選','Spot candidate','斑点候補'],region:['異常區域','Defect region','異常領域'],mixed:['混合候選','Mixed candidates','混合候補'],
 retention:['保留最近 30 天；未結案告警引用除外','30-day retention; open alerts retained','30日間保存。未確認警報の参照は保持'],
 algorithm:['演算法與參數','Algorithm and parameters','アルゴリズム・パラメータ'],alarms:['視覺檢查告警','Visual inspection alerts','外観検査警報'],noAlarms:['目前無未確認告警','No unacknowledged alerts','未確認警報なし'],imageError:['影像無法載入','Image unavailable','画像を読み込めません']
};
export default Object.fromEntries(['zh-TW','en','ja'].map((lang,i)=>[lang,Object.fromEntries(Object.entries(rows).map(([k,v])=>['vision_'+k,v[i]]))]));
