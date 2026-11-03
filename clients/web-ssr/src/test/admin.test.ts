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

  it('abre el radar por el gateway con el token en la query', () => {
    expect(wsAdminUrl('a.b.c', { protocol: 'http:', hostname: 'localhost' })).toBe(
      'ws://localhost:8080/admin/cluster/metrics?token=a.b.c',
    );
  });
});
