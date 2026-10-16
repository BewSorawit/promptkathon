import { z } from 'zod'

const stationSchema = z.object({
  id: z.number(), code: z.string(), name: z.string(), province: z.string(), district: z.string(), river: z.string(),
  latitude: z.number(), longitude: z.number(), bank_msl: z.number().nullable(),
  situation_level: z.number().nullable(), observed_at: z.string().nullable(),
  level_msl: z.number().nullable(), is_stale: z.boolean(),
})
const observationSchema = z.object({ observed_at: z.string(), level_msl: z.number(), source: z.string() })
const forecastSchema = z.object({
  origin_at: z.string(), target_at: z.string(), predicted_msl: z.number(),
  lower_msl: z.number(), upper_msl: z.number(), model_run_id: z.number(),
})
export type Station = z.infer<typeof stationSchema>
export type Observation = z.infer<typeof observationSchema>
export type ForecastResult = z.infer<typeof forecastSchema>

async function get<T>(path: string, schema: z.ZodType<T>): Promise<T> {
  const response = await fetch(path)
  if (!response.ok) throw new Error(`API ตอบกลับ ${response.status}`)
  return schema.parse(await response.json())
}
export const api = {
  stations: () => get('/api/v1/stations', z.object({ stations: z.array(stationSchema), source: z.string(), generated_at: z.string() })),
  observations: (id: number) => get(`/api/v1/stations/${id}/observations?hours=48`, z.object({ observations: z.array(observationSchema) })),
  forecast: (id: number) => get(`/api/v1/stations/${id}/forecast`, z.object({ status: z.enum(['available', 'unavailable', 'stale']), forecast: forecastSchema.nullable() })),
  source: () => get('/api/v1/source-status', z.object({ sync: z.object({ source: z.string(), attempted_at: z.string(), succeeded_at: z.string().nullable(), status: z.enum(['ok', 'error']), detail: z.string(), received_count: z.number() }).nullable() })),
}
