import {t,number,fixed} from './i18n.js';
export function utilizationText(metric,key){
 if(!metric)return t('loading');
 if(metric.quality==='no-data')return t('noData');
 if(metric.quality!=='complete'||!Number.isFinite(metric[key]))return t('incomplete');
 return `${fixed(metric[key],1)}%`;
}
