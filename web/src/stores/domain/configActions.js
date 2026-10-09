import { api } from '../../api/client';

export function createConfigActions(state, notify) {
  return {
    async loadConfig() {
      try {
        const data = await api.getConfig();
        state.config = {
          ...state.config,
          ...data,
          reasoner_provider: data.reasoner_provider || { api_base: '', api_key: '', model: '' },
          writer_provider: data.writer_provider || { api_base: '', api_key: '', model: '' },
          reviewer_provider: data.reviewer_provider || { api_base: '', api_key: '', model: '' },
        };
        state.enableReasonerProvider = Boolean(
          data.reasoner_provider && (data.reasoner_provider.api_base || data.reasoner_provider.api_key || data.reasoner_provider.model)
        );
        state.enableWriterProvider = Boolean(
          data.writer_provider && (data.writer_provider.api_base || data.writer_provider.api_key || data.writer_provider.model)
        );
        state.enableReviewerProvider = Boolean(
          data.reviewer_provider && (data.reviewer_provider.api_base || data.reviewer_provider.api_key || data.reviewer_provider.model)
        );
      } catch (e) {
        console.error('load config error:', e);
      }
    },

    async saveConfig() {
      try {
        const payload = { ...state.config };
        if (!state.enableReasonerProvider) {
          payload.reasoner_provider = null;
        }
        if (!state.enableWriterProvider) {
          payload.writer_provider = null;
        }
        if (!state.enableReviewerProvider) {
          payload.reviewer_provider = null;
        }
        await api.saveConfig(payload);
        notify('系统配置已保存', '模型与岗位参数已更新生效', 'success');
      } catch (e) {
        notify('保存配置失败', e.message, 'error');
      }
    },

    async testConfigConnection(target = 'default') {
      if (!state.configTestStatus) {
        state.configTestStatus = {};
      }
      state.configTestStatus[target] = { loading: true, status: null, message: '正在诊断连接...', latency_ms: 0 };

      const payload = { target };
      if (target === 'default') {
        payload.api_base = state.config.api_base;
        payload.api_key = state.config.api_key;
        payload.model = state.config.reasoning_model || state.config.writer_model;
      } else if (target === 'reasoner') {
        if (state.enableReasonerProvider && state.config.reasoner_provider) {
          payload.api_base = state.config.reasoner_provider.api_base || state.config.api_base;
          payload.api_key = state.config.reasoner_provider.api_key || state.config.api_key;
          payload.model = state.config.reasoner_provider.model || state.config.reasoning_model;
        } else {
          payload.api_base = state.config.api_base;
          payload.api_key = state.config.api_key;
          payload.model = state.config.reasoning_model;
        }
      } else if (target === 'writer') {
        if (state.enableWriterProvider && state.config.writer_provider) {
          payload.api_base = state.config.writer_provider.api_base || state.config.api_base;
          payload.api_key = state.config.writer_provider.api_key || state.config.api_key;
          payload.model = state.config.writer_provider.model || state.config.writer_model;
        } else {
          payload.api_base = state.config.api_base;
          payload.api_key = state.config.api_key;
          payload.model = state.config.writer_model;
        }
      } else if (target === 'reviewer') {
        if (state.enableReviewerProvider && state.config.reviewer_provider) {
          payload.api_base = state.config.reviewer_provider.api_base || state.config.api_base;
          payload.api_key = state.config.reviewer_provider.api_key || state.config.api_key;
          payload.model = state.config.reviewer_provider.model || state.config.reviewer_model;
        } else {
          payload.api_base = state.config.api_base;
          payload.api_key = state.config.api_key;
          payload.model = state.config.reviewer_model;
        }
      }

      try {
        const res = await api.testConfig(payload);
        if (res.status === 'ok') {
          state.configTestStatus[target] = {
            loading: false,
            status: 'ok',
            latency_ms: res.latency_ms,
            message: `响应正常 (${res.latency_ms}ms) | 模型: ${res.model}`,
          };
          notify('模型连接成功', `[${res.model}] 响应正常，延迟 ${res.latency_ms}ms`, 'success');
        } else {
          state.configTestStatus[target] = {
            loading: false,
            status: 'error',
            latency_ms: res.latency_ms,
            message: res.error || '连接失败',
          };
          notify('模型连接失败', res.error || '未能成功获取模型响应', 'error');
        }
      } catch (err) {
        state.configTestStatus[target] = {
          loading: false,
          status: 'error',
          latency_ms: 0,
          message: err.message || '网络连接异常',
        };
        notify('测试发生异常', err.message, 'error');
      }
    },
  };
}
