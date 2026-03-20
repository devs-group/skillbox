import { Configuration, FrontendApi } from "@ory/client"

const kratosUrl = process.env.NEXT_PUBLIC_KRATOS_URL || "http://localhost:4433"

export const ory = new FrontendApi(
  new Configuration({
    basePath: kratosUrl,
    baseOptions: { withCredentials: true },
  })
)
