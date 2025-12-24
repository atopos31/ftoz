import { ref } from 'vue'
import { defineStore } from 'pinia'
import axios from 'axios'

import { HOST, IS_DEV, USER_CONFIG_PATH } from '@/utils/env'

import { useLikeStore } from '@/store/like'

interface LikeModel {
  dir: string[]
}

const getDef = (): LikeModel => ({
  dir: IS_DEV ? ['/Users/flex/Downloads'] : ['/vol1/1000'],
})

export const useUserStore = defineStore('user', () => {
  const like = useLikeStore()

  const initialized = ref(false)

  const cfg = ref(getDef())

  const load = async () => {
    const { data: result1 } = await axios.get(HOST, {
      params: { _api: 'read', path: USER_CONFIG_PATH },
    })

    if (result1.code === 404) {
      await update()
    } else {
      cfg.value = result1 as LikeModel
    }

    like.cfg.folderActive = like.cfg.folderDefOpen || cfg.value.dir[0] || ''

    initialized.value = true
  }

  const update = async () => {
    await axios.post(
      HOST,
      {
        encode: 'utf8',
        path: USER_CONFIG_PATH,
        value: JSON.stringify(cfg.value),
        force: 1,
      },
      {
        params: { _api: 'save' },
      },
    )
  }

  return { initialized, cfg, load, update }
})
