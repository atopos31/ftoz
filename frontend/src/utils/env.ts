export const IS_DEV = import.meta.env.MODE === 'development'

export const HOST = IS_DEV ? 'http://127.0.0.1:17746' : '/cgi/ThirdParty/ftoz/index.cgi'

export const USER_CONFIG_PATH = IS_DEV
  ? '/Users/flex/Downloads/config.json'
  : '/var/apps/ftoz/shares/ftoz/config.json'


export const MIGRATE_URL = IS_DEV
  ? 'http://127.0.0.1:17746/migrate'
  : '/cgi/ThirdParty/ftoz/index.cgi?_api=migrate'

export const STATUS_URL = IS_DEV
  ? 'http://127.0.0.1:17746/status'
  : '/cgi/ThirdParty/ftoz/index.cgi?_api=status'
