;; The sample shop's products. seed_catalogue runs this with bb and prints how many there are.
;; Ref: https://book.babashka.org/
(def products
  [{:sku "MUG-1" :name "Mug" :price 12}
   {:sku "TEE-1" :name "T-shirt" :price 25}
   {:sku "CAP-1" :name "Cap" :price 18}])

(println (count products))
