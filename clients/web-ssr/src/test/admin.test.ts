import { describe, expect, it } from 'vitest';
import { toPutBody, wsAdminUrl } from '../views/Admin';

describe('consola de administración', () => {
  it('arma el cuerpo del PUT con la versión que se está editando', () => {
    const values = {
      jokers: {
        banner: { enabled: true, chips_per_discard: 45 },
        classic: { enabled: true, mult_bonus: 4 },
        duo: { enabled: false, mult_factor: 2 },
      },
      happy_hour: { enabled: true, global_mult: 2 },
    };
    expect(toPutBody(3, values)).toEqual({ expected_version: 3, ...values });
  });

  it('abre el radar por el mismo origen (wss en https, ws en http) con el token en la query', () => {
    expect(wsAdminUrl('a.b.c', { protocol: 'https:', host: 'localhost' }, '')).toBe(
      'wss://localhost/admin/cluster/metrics?token=a.b.c',
    );
    expect(wsAdminUrl('a.b.c', { protocol: 'http:', host: 'localhost' }, '')).toBe(
      'ws://localhost/admin/cluster/metrics?token=a.b.c',
    );
    expect(wsAdminUrl('a.b.c', { protocol: 'https:', host: 'localhost:8443' }, '')).toBe(
      'wss://localhost:8443/admin/cluster/metrics?token=a.b.c',
    );
    expect(wsAdminUrl('a b', { protocol: 'https:', host: 'localhost' }, 'wss://gw:9000/')).toBe(
      'wss://gw:9000/admin/cluster/metrics?token=a%20b',
    );
  });
});
