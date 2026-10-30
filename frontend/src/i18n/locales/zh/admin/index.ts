import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import accountHealth from './accountHealth'
import finance from './finance'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'
import plugins from './plugins'

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...accountHealth,
  ...finance,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
  ...plugins,
}
