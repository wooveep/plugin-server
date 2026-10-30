# Moderations

> Source: https://developers.openai.com/api/reference/resources/moderations
> Fetched: 2026-04-23

Given text and/or image inputs, classifies if those inputs are potentially harmful.

##### [Create moderation](https://developers.openai.com/api/reference/resources/moderations/methods/create)

POST/moderations

##### Models

Moderation object { categories, category\_applied\_input\_types, category\_scores, flagged }

categories: object { harassment, "harassment/threatening", hate, 10 more }

A list of the categories, and whether they are flagged or not.

harassment: boolean

Content that expresses, incites, or promotes harassing language towards any target.

"harassment/threatening": boolean

Harassment content that also includes violence or serious harm towards any target.

hate: boolean

Content that expresses, incites, or promotes hate based on race, gender, ethnicity, religion, nationality, sexual orientation, disability status, or caste. Hateful content aimed at non-protected groups (e.g., chess players) is harassment.

"hate/threatening": boolean

Hateful content that also includes violence or serious harm towards the targeted group based on race, gender, ethnicity, religion, nationality, sexual orientation, disability status, or caste.

illicit: boolean

Content that includes instructions or advice that facilitate the planning or execution of wrongdoing, or that gives advice or instruction on how to commit illicit acts. For example, “how to shoplift” would fit this category.

"illicit/violent": boolean

Content that includes instructions or advice that facilitate the planning or execution of wrongdoing that also includes violence, or that gives advice or instruction on the procurement of any weapon.

"self-harm": boolean

Content that promotes, encourages, or depicts acts of self-harm, such as suicide, cutting, and eating disorders.

"self-harm/instructions": boolean

Content that encourages performing acts of self-harm, such as suicide, cutting, and eating disorders, or that gives instructions or advice on how to commit such acts.

"self-harm/intent": boolean

Content where the speaker expresses that they are engaging or intend to engage in acts of self-harm, such as suicide, cutting, and eating disorders.

sexual: boolean

Content meant to arouse sexual excitement, such as the description of sexual activity, or that promotes sexual services (excluding sex education and wellness).

"sexual/minors": boolean

Sexual content that includes an individual who is under 18 years old.

violence: boolean

Content that depicts death, violence, or physical injury.

"violence/graphic": boolean

Content that depicts death, violence, or physical injury in graphic detail.

category\_applied\_input\_types: object { harassment, "harassment/threatening", hate, 10 more }

A list of the categories along with the input type(s) that the score applies to.

harassment: array of "text"

The applied input type(s) for the category ‘harassment’.

"harassment/threatening": array of "text"

The applied input type(s) for the category ‘harassment/threatening’.

hate: array of "text"

The applied input type(s) for the category ‘hate’.

"hate/threatening": array of "text"

The applied input type(s) for the category ‘hate/threatening’.

illicit: array of "text"

The applied input type(s) for the category ‘illicit’.

"illicit/violent": array of "text"

The applied input type(s) for the category ‘illicit/violent’.

"self-harm": array of "text" or "image"

The applied input type(s) for the category ‘self-harm’.

One of the following:

"text"

"image"

"self-harm/instructions": array of "text" or "image"

The applied input type(s) for the category ‘self-harm/instructions’.

One of the following:

"text"

"image"

"self-harm/intent": array of "text" or "image"

The applied input type(s) for the category ‘self-harm/intent’.

One of the following:

"text"

"image"

sexual: array of "text" or "image"

The applied input type(s) for the category ‘sexual’.

One of the following:

"text"

"image"

"sexual/minors": array of "text"

The applied input type(s) for the category ‘sexual/minors’.

violence: array of "text" or "image"

The applied input type(s) for the category ‘violence’.

One of the following:

"text"

"image"

"violence/graphic": array of "text" or "image"

The applied input type(s) for the category ‘violence/graphic’.

One of the following:

"text"

"image"

category\_scores: object { harassment, "harassment/threatening", hate, 10 more }

A list of the categories along with their scores as predicted by model.

harassment: number

The score for the category ‘harassment’.

"harassment/threatening": number

The score for the category ‘harassment/threatening’.

hate: number

The score for the category ‘hate’.

"hate/threatening": number

The score for the category ‘hate/threatening’.

illicit: number

The score for the category ‘illicit’.

"illicit/violent": number

The score for the category ‘illicit/violent’.

"self-harm": number

The score for the category ‘self-harm’.

"self-harm/instructions": number

The score for the category ‘self-harm/instructions’.

"self-harm/intent": number

The score for the category ‘self-harm/intent’.

sexual: number

The score for the category ‘sexual’.

"sexual/minors": number

The score for the category ‘sexual/minors’.

violence: number

The score for the category ‘violence’.

"violence/graphic": number

The score for the category ‘violence/graphic’.

flagged: boolean

Whether any of the below categories are flagged.

ModerationModel = "omni-moderation-latest" or "omni-moderation-2024-09-26" or "text-moderation-latest" or "text-moderation-stable"

One of the following:

"omni-moderation-latest"

"omni-moderation-2024-09-26"

"text-moderation-latest"

"text-moderation-stable"

ModerationCreateResponse object { id, model, results }

Represents if a given text input is potentially harmful.

id: string

The unique identifier for the moderation request.

model: string

The model used to generate the moderation results.

results: array of [Moderation](https://developers.openai.com/api/reference/resources/moderations#(resource)%20moderations%20%3E%20(model)%20moderation%20%3E%20(schema)) { categories, category\_applied\_input\_types, category\_scores, flagged }

A list of moderation objects.

categories: object { harassment, "harassment/threatening", hate, 10 more }

A list of the categories, and whether they are flagged or not.

harassment: boolean

Content that expresses, incites, or promotes harassing language towards any target.

"harassment/threatening": boolean

Harassment content that also includes violence or serious harm towards any target.

hate: boolean

Content that expresses, incites, or promotes hate based on race, gender, ethnicity, religion, nationality, sexual orientation, disability status, or caste. Hateful content aimed at non-protected groups (e.g., chess players) is harassment.

"hate/threatening": boolean

Hateful content that also includes violence or serious harm towards the targeted group based on race, gender, ethnicity, religion, nationality, sexual orientation, disability status, or caste.

illicit: boolean

Content that includes instructions or advice that facilitate the planning or execution of wrongdoing, or that gives advice or instruction on how to commit illicit acts. For example, “how to shoplift” would fit this category.

"illicit/violent": boolean

Content that includes instructions or advice that facilitate the planning or execution of wrongdoing that also includes violence, or that gives advice or instruction on the procurement of any weapon.

"self-harm": boolean

Content that promotes, encourages, or depicts acts of self-harm, such as suicide, cutting, and eating disorders.

"self-harm/instructions": boolean

Content that encourages performing acts of self-harm, such as suicide, cutting, and eating disorders, or that gives instructions or advice on how to commit such acts.

"self-harm/intent": boolean

Content where the speaker expresses that they are engaging or intend to engage in acts of self-harm, such as suicide, cutting, and eating disorders.

sexual: boolean

Content meant to arouse sexual excitement, such as the description of sexual activity, or that promotes sexual services (excluding sex education and wellness).

"sexual/minors": boolean

Sexual content that includes an individual who is under 18 years old.

violence: boolean

Content that depicts death, violence, or physical injury.

"violence/graphic": boolean

Content that depicts death, violence, or physical injury in graphic detail.

category\_applied\_input\_types: object { harassment, "harassment/threatening", hate, 10 more }

A list of the categories along with the input type(s) that the score applies to.

harassment: array of "text"

The applied input type(s) for the category ‘harassment’.

"harassment/threatening": array of "text"

The applied input type(s) for the category ‘harassment/threatening’.

hate: array of "text"

The applied input type(s) for the category ‘hate’.

"hate/threatening": array of "text"

The applied input type(s) for the category ‘hate/threatening’.

illicit: array of "text"

The applied input type(s) for the category ‘illicit’.

"illicit/violent": array of "text"

The applied input type(s) for the category ‘illicit/violent’.

"self-harm": array of "text" or "image"

The applied input type(s) for the category ‘self-harm’.

One of the following:

"text"

"image"

"self-harm/instructions": array of "text" or "image"

The applied input type(s) for the category ‘self-harm/instructions’.

One of the following:

"text"

"image"

"self-harm/intent": array of "text" or "image"

The applied input type(s) for the category ‘self-harm/intent’.

One of the following:

"text"

"image"

sexual: array of "text" or "image"

The applied input type(s) for the category ‘sexual’.

One of the following:

"text"

"image"

"sexual/minors": array of "text"

The applied input type(s) for the category ‘sexual/minors’.

violence: array of "text" or "image"

The applied input type(s) for the category ‘violence’.

One of the following:

"text"

"image"

"violence/graphic": array of "text" or "image"

The applied input type(s) for the category ‘violence/graphic’.

One of the following:

"text"

"image"

category\_scores: object { harassment, "harassment/threatening", hate, 10 more }

A list of the categories along with their scores as predicted by model.

harassment: number

The score for the category ‘harassment’.

"harassment/threatening": number

The score for the category ‘harassment/threatening’.

hate: number

The score for the category ‘hate’.

"hate/threatening": number

The score for the category ‘hate/threatening’.

illicit: number

The score for the category ‘illicit’.

"illicit/violent": number

The score for the category ‘illicit/violent’.

"self-harm": number

The score for the category ‘self-harm’.

"self-harm/instructions": number

The score for the category ‘self-harm/instructions’.

"self-harm/intent": number

The score for the category ‘self-harm/intent’.

sexual: number

The score for the category ‘sexual’.

"sexual/minors": number

The score for the category ‘sexual/minors’.

violence: number

The score for the category ‘violence’.

"violence/graphic": number

The score for the category ‘violence/graphic’.

flagged: boolean

Whether any of the below categories are flagged.
