import { QueryClient } from "@tanstack/react-query"
import { describe, expect, it } from "vitest"

import { cacheAuthenticated } from "./auth"
import type { Me } from "./client"

describe("cacheAuthenticated", () => {
  it("同步 me 与 auth-status，登录后立即视为已初始化", () => {
    const qc = new QueryClient()
    const me: Me = { username: "liang", is_owner: true, via_pat: false }
    cacheAuthenticated(qc, me)
    expect(qc.getQueryData(["me"])).toEqual(me)
    expect(qc.getQueryData(["auth-status"])).toEqual({
      initialized: true,
      username: "liang",
    })
  })
})
